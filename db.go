package main

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

// sample key
// "sha256:dc3791a61558d7758d9e9b0dc0b83b1d85f4c46b52ec9f0ba5bbefdcf98ba6e6_layer_file": {
// 	"path": "/tmp/images/caches/ref/sha256:dc3791a61558d7758d9e9b0dc0b83b1d85f4c46b52ec9f0ba5bbefdcf98ba6e6_layer_file",
// 	"size": 2131,
// 	"ref_cnt": 0,                                         <======
// 	"ref_last": "2024-03-20T11:07:52.747795791-07:00"     <======
//   }

// There is another high-level structure and it might keep changing whenever it is referenced (read)...... The frequency is very high.
// They are the key elements to purge entries.

type CacheItem struct {
	ID         int
	Key        string
	Value      string
	ValueB     []byte
	Size       int
	RefCount   int
	RefLast    int64
	Compressed bool
}

var dbHandle *sql.DB = nil
var dbFile = "./scancache.db"
var tableName = "cache"

func createDb() error {
	// delete existing file
	if _, err := os.Stat(dbFile); err == nil {
		err := os.Remove(dbFile)
		if err != nil {
			return err
		}
	}

	// create file based db
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return err
	}
	dbHandle = db

	columns := []string{
		"id INTEGER NOT NULL PRIMARY KEY",
		"key TEXT UNIQUE",
		"valuet TEXT",
		"valueb BLOB",
		"size INTEGER",
		"ref_cnt INTEGER",
		"ref_last INTEGER",
	}

	statements := make([]string, 0)
	statements = append(statements, fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", tableName, strings.Join(columns, ",")))
	statements = append(statements, fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_key_idx on %s (key)", tableName, tableName))
	statements = append(statements, fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_key_idx on %s (ref_last)", tableName, tableName))

	for _, oneSql := range statements {
		_, err = dbHandle.Exec(oneSql)
		if err != nil {
			return err
		}
	}

	return nil
}

func openDb() error {
	_, err := os.Stat(dbFile)
	if err != nil {
		return err
	}

	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return err
	}
	dbHandle = db
	return nil
}

func CreateCacheItem(item CacheItem) error {
	if item.Compressed {
		if len(item.Value) > 0 {
			var buf bytes.Buffer
			enc := gob.NewEncoder(&buf)
			if err := enc.Encode(&item.Value); err == nil {
				item.ValueB = GzipBytes(buf.Bytes())
			}
		}

		_, err := dbHandle.Exec("INSERT INTO cache (key, valueb, size, ref_cnt, ref_last) VALUES (?, ?, ?, ?, ?)",
			item.Key, item.ValueB, item.Size, item.RefCount, item.RefLast)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				return UpdateCacheItemByKey(item)
			}
		}

		return nil
	}

	// no compression
	_, err := dbHandle.Exec("INSERT INTO cache (key, valuet, size, ref_cnt, ref_last) VALUES (?, ?, ?, ?, ?)",
		item.Key, item.Value, item.Size, item.RefCount, item.RefLast)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return UpdateCacheItemByKey(item)
		}
	}
	return err
}

func ReadCacheItemByKey(key string, compression bool) (CacheItem, error) {
	if compression {
		var item CacheItem
		err := dbHandle.QueryRow("SELECT id, key, valueb, size, ref_cnt, ref_last FROM cache WHERE key = ?", key).
			Scan(&item.ID, &item.Key, &item.ValueB, &item.Size, &item.RefCount, &item.RefLast)

		// unzip
		if uzb := GunzipBytes(item.ValueB); uzb != nil {
			var dataObj string // when we do zip, it's in string...
			buf := bytes.NewBuffer(uzb)
			dec := gob.NewDecoder(buf)
			if err := dec.Decode(&dataObj); err == nil {
				item.Value = dataObj
			}
		}

		return item, err
	}

	// no compression
	var item CacheItem
	err := dbHandle.QueryRow("SELECT id, key, valuet, size, ref_cnt, ref_last FROM cache WHERE key = ?", key).
		Scan(&item.ID, &item.Key, &item.Value, &item.Size, &item.RefCount, &item.RefLast)
	return item, err
}

func ReadCacheItemById(db *sql.DB, id int) (CacheItem, error) {
	var item CacheItem
	err := db.QueryRow("SELECT id, key, size, ref_cnt, ref_last FROM cache WHERE id = ?", id).
		Scan(&item.ID, &item.Key, &item.Size, &item.RefCount, &item.RefLast)
	return item, err
}

func UpdateCacheItemByKey(item CacheItem) error {
	if item.Compressed {
		_, err := dbHandle.Exec("UPDATE cache SET valueb = ?, size = ?, ref_cnt = ?, ref_last = ? WHERE key = ?",
			item.ValueB, item.Size, item.RefCount, item.RefLast, item.Key)
		return err
	}

	// no compression
	_, err := dbHandle.Exec("UPDATE cache SET valuet = ?, size = ?, ref_cnt = ?, ref_last = ? WHERE key = ?",
		item.Value, item.Size, item.RefCount, item.RefLast, item.Key)
	return err
}

func UpdateCacheItemRefCountById(id, refCnt int) error {
	_, err := dbHandle.Exec("UPDATE cache SET ref_cnt = ? WHERE id = ?",
		refCnt, id)
	return err
}

func DeleteBatchRecordsByRefLast(ref_last_start, ref_last_end int) error {
	_, err := dbHandle.Exec("delete from cache where ref_last>=? and ref_last<=?", ref_last_start, ref_last_end)
	return err
}

func DeleteCacheItemByKey(key string) error {
	_, err := dbHandle.Exec("DELETE FROM cache WHERE key = ?", key)
	return err
}

func GzipBytes(buf []byte) []byte {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	w.Write(buf)
	w.Close()

	return b.Bytes()
}

func GunzipBytes(buf []byte) []byte {
	b := bytes.NewBuffer(buf)
	r, err := gzip.NewReader(b)
	if err != nil {
		return nil
	}
	defer r.Close()
	uzb, _ := ioutil.ReadAll(r)
	return uzb
}
