package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"math/rand"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var useCompression = true

func main() {
	// ./scancache -action create -count 2000
	// ./scancache -action read -count 1000
	// ./scancache -action update -count 1000

	action := flag.String("action", "", "Action to perform (create, read, update, delete (ttl_delete) and search)")
	count := flag.Int("count", 1000, "Number of times to perform the action")
	search_key := flag.String("key", "", "Key to search")

	flag.Parse()

	if *action == "" || *count == 0 {
		flag.PrintDefaults()
		return
	}

	switch *action {
	case "create":
		fmt.Printf("Create %d records\n", *count)
		actionCreate(*count)
	case "read":
		fmt.Printf("Random read action %d times\n", *count)
		actionRead(*count)
	case "update":
		fmt.Printf("Random update action %d times\n", *count)
		actionUpdate(*count)
	case "delete":
		fmt.Printf("Random delete action %d times\n", *count)
		actionDelete(*count)
	case "search":
		fmt.Printf("Search by key '%s'\n", *search_key)
		actionSearch(*search_key)
	case "ttl_delete":
		// for 3M db
		ref_last_start := 1711208020
		ref_last_end := 1711208450

		// for 1M db
		// 1711168520 and ref_last<=1711168720
		ref_last_start = 1711168520
		ref_last_end = 1711168720

		// for 500K db
		//  sqlite3 scancache.db 'select count(*) from cache where ref_last>=1711221800 and ref_last<=1711221930'
		ref_last_start = 1711221800
		ref_last_end = 1711221930

		// for 100K db
		//   sqlite3 scancache.db 'select count(*) from cache where ref_last>=1711157140 and ref_last<=1711157260'
		ref_last_start = 1711157140
		ref_last_end = 1711157260

		actionTTLDelete(ref_last_start, ref_last_end)
	default:
		fmt.Println("❌ Invalid action provided")
		flag.PrintDefaults()
	}
}

func actionTTLDelete(ref_last_start, ref_last_end int) {
	openDb()

	err := DeleteBatchRecordsByRefLast(ref_last_start, ref_last_end)
	if err != nil {
		fmt.Println("❌ Error creating cache item:", err)
		return
	}
	fmt.Printf("✔️ Done. Records with ref_last ranges from %d to %d is deleted.\n", ref_last_start, ref_last_end)
}

func actionCreate(count int) {
	// create db
	err := createDb()
	if err != nil {
		fmt.Printf("initDb failed, error = %s\n", err)
		return
	}

	for i := 0; i < count; i++ {
		data := GenerateRandomJSON()
		key_sha256 := calculateSHA256(data)

		currentTime := time.Now()
		unixTime := currentTime.Unix()

		item := CacheItem{Key: key_sha256, Value: data, Size: 10, RefCount: 1, RefLast: unixTime, Compressed: useCompression}
		startTime := time.Now()
		err := CreateCacheItem(item)
		if err != nil {
			fmt.Println("❌ Error creating cache item:", err)
			return
		}
		endTime := time.Now()
		elapsedTime := endTime.Sub(startTime)

		if i%1000 == 0 {
			fmt.Printf("\t%d/%d.., took %v\n", i, count, elapsedTime)
		}
	}

	fmt.Printf("Done. Create %d records\n", count)
}

func actionRead(count int) {
	keys := getRandomKeys(count)

	openDb()

	for i, key := range keys {
		startTime := time.Now()
		c, err := ReadCacheItemByKey(key, useCompression)
		if err != nil {
			fmt.Printf("❌ fetch data failed, key=%s, error=%v\n", key, err)
			return
		}
		endTime := time.Now()
		elapsedTime := endTime.Sub(startTime)

		fmt.Printf("[%d] fetch key=%s, RefCount=%d, value_length (bytes)=%d, time=%v\n", i, key, c.RefCount, len(c.Value), elapsedTime)
	}
}

func actionUpdate(count int) {
	keys := getRandomKeys(count)

	openDb()

	startTime := time.Now()

	for i, key := range keys {
		// step-1: read data back
		c, err := ReadCacheItemByKey(key, useCompression)
		if err != nil {
			fmt.Printf("fetch data failed, key=%s, error=%v\n", key, err)
			return
		}

		if i%100 == 0 {
			fmt.Printf("[%d] fetch key=%s, RefCount=%d\n", i, key, c.RefCount)
		}

		// step-2: increase ref_cnt
		c.RefCount++

		// step-3: write back
		err = UpdateCacheItemRefCountById(c.ID, c.RefCount)
		if err != nil {
			fmt.Println("Error updating cache item:", err)
			return
		}
	}
	fmt.Println("==================================")

	endTime := time.Now()
	elapsedTime := endTime.Sub(startTime)

	// read back
	for i, key := range keys {
		c, err := ReadCacheItemByKey(key, useCompression)
		if err != nil {
			fmt.Printf("fetch data failed, key=%s, error=%v\n", key, err)
			return
		}

		if i%100 == 0 {
			fmt.Printf("\t[%d] fetch key=%s, RefCount=%d\n", i, key, c.RefCount)
		}
	}

	averageTime := elapsedTime / time.Duration(count)
	fmt.Printf("👉 >> Done. update ref_cnt average time: %v (total_time=%v, count=%d)\n", averageTime, elapsedTime, count)
}

func actionDelete(count int) {
	keys := getRandomKeys(count)

	openDb()

	startTime := time.Now()
	for i, key := range keys {
		err := DeleteCacheItemByKey(key)
		if err != nil {
			fmt.Printf("delete key failed, key=%s, error=%v\n", key, err)
		}

		if i%100 == 0 {
			fmt.Printf("\t[%d] delete key=%s\n", i, key)
		}
	}
	endTime := time.Now()
	elapsedTime := endTime.Sub(startTime)

	averageTime := elapsedTime / time.Duration(count)
	fmt.Printf("👉 >> Done. delete record average time: %v (total_time=%v, count=%d)\n", averageTime, elapsedTime, count)
}

func actionSearch(key string) {

	// open db
	openDb()

	startTime := time.Now()
	c, err := ReadCacheItemByKey(key, useCompression)
	if err != nil {
		fmt.Printf("❌ fetch data failed, key=%s, error=%v\n", key, err)
		return
	}
	endTime := time.Now()
	elapsedTime := endTime.Sub(startTime)

	fmt.Printf("✔️ fetch key=%s, RefCount=%d, value_length (bytes)=%d, time=%v\n", key, c.RefCount, len(c.Value), elapsedTime)
}

func getRandomKeys(howMany int) []string {
	fmt.Printf("Pickup %d keys randomly...\n", howMany)

	var results []string
	// open database
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return results
	}
	defer db.Close()

	// get count
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM cache").Scan(&count)
	if err != nil {
		fmt.Println("❌ Error querying database:", err)
		return results
	}

	uniqueRandomSlice := generateUniqueRandomSlice(1, count, howMany)

	// get keys of these IDs
	for _, id := range uniqueRandomSlice {
		item, err := ReadCacheItemById(db, id)
		if err != nil {
			fmt.Printf("❌ Error querying row_id:%d, err:%v\n", id, err)
			// return results
			continue
		}
		results = append(results, item.Key)
	}

	return results
}

func generateUniqueRandomSlice(min, max, count int) []int {
	if count > (max - min + 1) {
		count = max - min + 1
	}

	rand.Seed(time.Now().UnixNano())

	numbers := make(map[int]bool)
	var result []int

	for len(result) < count {
		num := rand.Intn(max-min+1) + min
		if !numbers[num] {
			numbers[num] = true
			result = append(result, num)
		}
	}

	return result
}

func calculateSHA256(input string) string {
	data := []byte(input)
	hash := sha256.Sum256(data)
	hashString := hex.EncodeToString(hash[:])

	return hashString
}
