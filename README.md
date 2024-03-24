# scanner cache option evalution

### History

- v1 - 2024/03/22, initial version

## Table of Contents

- [Section 1: Background](#section-1-background)
- [Section 2: Test Result](#section-2-test-result)
- [Section 3: Database Schema](#section-3-database-schema)
- [Section 4: Test Program Usage](#section-4-test-program-usage)

## Section 1: Background

Below are several quotes from Slack.

- we are thinking about add a database into scanner to cache layer scan data.
- The characteristics of the scanner cache are file-based and KV database. No fancy "select" with "criteria" operations.
- I think memory usage is a concern, if we load the data into the memory
- disk access probably fast enough for scanning purpose, the downside of current approach is there are a lot of files
  - Yes, the only issue now is the file allocations on the (N\*4KB). Some file space might be spoiled.
- There is another high-level structure and it might keep changing whenever it is referenced (read)...... The frequency is very high.
  - the ref_cnt and ref_last are meta data of this key, and it need to be changed quite frequent, right? like ref_cnt++

> [!NOTE]
> I presume that the data isn't shared across Pods/Nodes. If there's a need to share data among Pods and across Nodes, it's not practical to use SQLite directly.

Compose a sample program demonstrating the insertion, updating, and retrieval of data in SQLite. Include functionality to simulate a substantial load, such as millions of records, and gather performance data.

## Section 2: Test Result

Environment: lab VM (10.1.45.44) , Ubuntu 22.04

| Records count | File size | 1️⃣ Action - Insert a record | 2️⃣ Action - Get a record | 3️⃣ Action - Update a record | 4️⃣ Action - Delete a record |
| ------------- | --------- | --------------------------- | ------------------------ | --------------------------- | --------------------------- |
| 100 K         | 57MB      | < 8 ms                      | < 3 ms                   | < 7 ms                      | < 6 ms                      |
| 500 K         | 288MB     | < 10 ms                     | < 3 ms                   | < 7 ms                      | < 10 ms                     |
| 1 M           | 577MB     | < 12 ms                     | < 3 ms                   | < 7 ms                      | < 13 ms                     |
| 3 M           | 1.73GB    | < 20 ms                     | < 4 ms                   | < 18 ms                     | < 28 ms                     |

---

| Records count | File size | 5️⃣ Action - Batch delete TTL records |
| ------------- | --------- | ------------------------------------ |
| 100 K         | 57MB      | < 4 sec, 21145 rows                  |
| 500 K         | 288MB     | < 12 sec, 21566 rows                 |
| 1 M           | 577MB     | < 23 sec, 21773 rows                 |
| 3 M           | 1.73GB    | < 45 sec, 20193 rows                 |

> Delete '20193' records from a database containing '3M' records.

---

2️⃣ **Get a record** : this process involves searching for the record by its key, obtaining it, and decompressing its value column.

3️⃣ **Update a record**: this process involves searching for it by key, retrieving its ref_cnt value, incrementing it by one, and then saving the updated record back to the database.

4️⃣ **Delete a record**: this process involves searching for it by key, and then delete it from database.

5️⃣ **Batch delete TTL records**
this process involves searching for the last reference time of a record (in the `ref_last` column) and deleting the record if it falls within a specific range.

```
sqlite3 scancache.db 'select count(\*) from cache where ref_last>=1711168520 and ref_last<=1711168720'
```

> [!NOTE]
> memory and disk usage measurements are not available in this initial version. We can conduct these measurements in a later stage.

### What does a record contain?

Each record comprises the following components, culminating in a total size of approximately 840 bytes per record.

1. key (64 bytes)
2. value (~760 bytes, will be compressed, achieving a compression rate of about 50%). Some fields in the JSON contain randomly generated data.
3. meta data, like ref_cnt, updated_time_stamp, etc.

Following JSON is a sample value we'll insert, with a size of approximately 760 bytes.

<details><summary>Sample value</summary>

```
{
  "secrets": [
    {
      "Type": "regular",
      "Text": "goodPasswd : \"A)8hKd]xrcA33^6_...",
      "File": "/Credential.yaml",
      "RuleDesc": "Credential",
      "Suggestion": "Please cloak your password and secret key"
    },
    {
      "Type": "regular",
      "Text": "password : \"A)8hKd]xrcA33^6__B...",
      "File": "/Credential1.yaml",
      "RuleDesc": "Credential",
      "Suggestion": "Please cloak your password and secret key"
    }
  ],
  "set_ids": [
    {
      "Type": "setgid",
      "File": "/var/log/apache2",
      "Evidence": "dgrwxr-xr-x"
    },
    {
      "Type": "setgid",
      "File": "/var/www/localhost/htdocs",
      "Evidence": "dgrwxr-xr-x"
    },
    {
      "Type": "setuid",
      "File": "/usr/sbin/suexec",
      "Evidence": "urwxr-xr-x"
    }
  ]
}
```

</details>

## Section 3: Database Schema

Index is created on the key column.

```
jeff@ub2204:~/myprojects/scanner-cache$ sqlite3 scancache.db .schema
CREATE TABLE cache (id INTEGER NOT NULL PRIMARY KEY,
                    key TEXT UNIQUE,
                    valueb BLOB,     👈  this will store the compressed JSON data
                    size INTEGER,
                    ref_cnt INTEGER,
                    ref_last INTEGER);
```

## Section 4: Test Program Usage

**Usage of the CLI**

<details><summary>Insert - Generate records by creating random data and inserting it into the database.</summary>

```
jeff@ub2204:~/myprojects/scanner-cache$ ./scancache -action create -count 10000
Create 10000 records
0/10000.., took 6.90177ms
1000/10000.., took 5.558501ms
2000/10000.., took 6.296078ms
3000/10000.., took 6.202281ms
4000/10000.., took 5.435751ms
5000/10000.., took 5.618432ms
6000/10000.., took 6.152327ms
7000/10000.., took 5.837ms
8000/10000.., took 5.647423ms
9000/10000.., took 6.731134ms
Done. Create 10000 records
```

</details>

<details><summary>Read - Retrieve specific keys from the database and measure the time it takes.</summary>

```
jeff@ub2204:~/myprojects/scanner-cache$ ./scancache -action read -count 10
Random read action 10 times
Pickup 10 keys randomly...
[0] fetch key=d06b229fb0888da603652a9444161cf33edacb79e0c4b958c30510d3a08d9598, RefCount=1, value_length (bytes)=770, time=221.114µs
[1] fetch key=f94efbc06d140c293887944e22e27c049935550d4954a0655f26e57d09449e1f, RefCount=1, value_length (bytes)=760, time=90.965µs
[2] fetch key=2c04584700cdbe930d5d5a3c0240dbf9e663d1738f5f4dad48f9aa544e7b0b43, RefCount=1, value_length (bytes)=775, time=89.368µs
[3] fetch key=ae4aca69ed891c28065c24fc36d2c4bca93dc9774e198fb109c0b6d0764dc75b, RefCount=1, value_length (bytes)=765, time=536.731µs
[4] fetch key=372ef36be2356a8d384f7e2c3a73e5481993b4e5f3a967488a67c0509e582d36, RefCount=1, value_length (bytes)=760, time=106.7µs
[5] fetch key=5daadcbc2030d6a0c3bc1fdfd37992310f794fdb64752b88c6aca0d2f1bdd5b9, RefCount=1, value_length (bytes)=765, time=68.121µs
[6] fetch key=4e59aef1f1ef1ffb3426c3a34b53195a95eff0ea5e2063eff9c72ef146f871b5, RefCount=1, value_length (bytes)=765, time=72.997µs
[7] fetch key=a6b1c96954326030a524a96eca80616331a5deb487b4270847fd423b3ec2e9fd, RefCount=1, value_length (bytes)=760, time=100.877µs
[8] fetch key=f57bd2e64322c9c5358f9fdeb6cfb69b1534b2edb9e1bb3ffdcc37cd60a7fcd0, RefCount=1, value_length (bytes)=765, time=67.51µs
[9] fetch key=daa34da06ad47d6b6b2e9e687612a255ec5aa5b946f64ff7c3d963cdcb0074a0, RefCount=1, value_length (bytes)=765, time=91.551µs
```

</details>

<details><summary>Update - Increase the ref_cnt values by 1 through a process involving retrieval and updating of the database.</summary>

```
jeff@ub2204:~/myprojects/scanner-cache$ ./scancache -action update  -count 1000
Random update action 1000 times
Pickup 1000 keys randomly...
[0] fetch key=0cb2c138764f9e58dea5ed77c0b7d8d82c5e55cff1cdab656e5f42a7bbc06624, RefCount=1
[100] fetch key=118cda255800cbea9979479f32a7d043445450f4986f8160a84134d617813aac, RefCount=1
[200] fetch key=1d834e9b5ad66fc64440951938b746f2f441fa12ac51d05a84f401fba1ff811e, RefCount=1
[300] fetch key=832b84e53a938218689113c61b11ef321daf92ce3086846e0ded110a83f6b094, RefCount=1
[400] fetch key=b94ce01da797e6dc6e3f946e7205e20fcbdb7510883ca29d0a24d116b9c4b7d0, RefCount=1
[500] fetch key=03a762f0609ed155fb9a74473e4910b52a2eb6dfb770cbd5ef9ba4f55c0a2dd8, RefCount=1
[600] fetch key=b8a2910ebdbf8a243bc16d81f813ee87054f86891e374718821cf296d32184f0, RefCount=1
[700] fetch key=32e240ed2277261bd0a6fd2eccdfaa355de0fec3038c5dcabd31cea2c3227773, RefCount=1
[800] fetch key=92712a6b0c699e8a0b6e832e7dbc419832e4729f843aeef37e19604295391df0, RefCount=1
[900] fetch key=e221901c93108f945c509f983524d4f9b50d66e35f7b6bd507078f48b7639bfe, RefCount=1
==================================
[0] fetch key=0cb2c138764f9e58dea5ed77c0b7d8d82c5e55cff1cdab656e5f42a7bbc06624, RefCount=2
[100] fetch key=118cda255800cbea9979479f32a7d043445450f4986f8160a84134d617813aac, RefCount=2
[200] fetch key=1d834e9b5ad66fc64440951938b746f2f441fa12ac51d05a84f401fba1ff811e, RefCount=2
[300] fetch key=832b84e53a938218689113c61b11ef321daf92ce3086846e0ded110a83f6b094, RefCount=2
[400] fetch key=b94ce01da797e6dc6e3f946e7205e20fcbdb7510883ca29d0a24d116b9c4b7d0, RefCount=2
[500] fetch key=03a762f0609ed155fb9a74473e4910b52a2eb6dfb770cbd5ef9ba4f55c0a2dd8, RefCount=2
[600] fetch key=b8a2910ebdbf8a243bc16d81f813ee87054f86891e374718821cf296d32184f0, RefCount=2
[700] fetch key=32e240ed2277261bd0a6fd2eccdfaa355de0fec3038c5dcabd31cea2c3227773, RefCount=2
[800] fetch key=92712a6b0c699e8a0b6e832e7dbc419832e4729f843aeef37e19604295391df0, RefCount=2
[900] fetch key=e221901c93108f945c509f983524d4f9b50d66e35f7b6bd507078f48b7639bfe, RefCount=2

>> Done. update ref_cnt average time: 4.590108ms (total_time=4.590108355s, count=1000)
```

</details>

<details><summary>Delete</summary>

```
neuvector@ubuntu2204-E:~/myprojects/scanner-cache$ ./scancache -action delete -count 1000
Random delete action 1000 times
Pickup 1000 keys randomly...
        [0] delete key=5b826a8316aadb6f2771fa2e5f2ffd3914754d8746ef18de0669f9c34c5c8739
        [100] delete key=c0e4c0fcaf0e7cca067d85afe60edc9a94b2b0555011596010dfedd9f458e7bf
        [200] delete key=497bc7a04723da9b532eca98045812245d9e56a032ee61d7552c76ed08a93a1f
        [300] delete key=c3a1681c3371f42a549778574f2edf5c28441d8e9b074dbaed5aa42eaa598acb
        [400] delete key=645d607630008f9f909e72704a59bb9134b4f7142cc4478c06d1e2a2cc6a7caf
        [500] delete key=c7378214a29688204d8a2388339bc7549bf2397c4b8829adb65db7304cad0873
        [600] delete key=b3137805da14bb20bea95c96c73bb42c36eb9ccaf160c6e65f5ea8fef2e85aee
        [700] delete key=cbdc6c1d45b7810a44972ee20448a61149f036c7e2b1a50853e0a8f57a4f54e2
        [800] delete key=41e6bad6ed1ef82ed3253a7c4d5a32d719c1b086e5eb5c36f69dd7cc49c9ae6c
        [900] delete key=6c2be5277f5837eb7a4106541cf96508fb2489c1edc9c7bb43c148eab459a156
👉 >> Done. delete record average time: 10.038323ms (total_time=10.038323449s, count=1000)
```

</details>

<details><summary>Search</summary>

```
Use sqlite cli to get some keys

jeff@ub2204:~/myprojects/scanner-cache$ sqlite3 scancache.db 'select key from cache limit 10'
000a9082db27daf54d52ea0969296174813b7584542ee3d7a256df977a6445dc
000c1700b47a8fe6ed9cb617419a65141fb0d6f3da8aee084d5739ddbfeb69a7
000dacd2e16ca4894dc1c8726518edae86cda1750d587f3e8ffc5691609099d4
00163599a05d2840b1b14853b442cd768406b01744adfe241152d3dab2fbaa58
00196b7cd9afd0aad1162624e7abb4a174aa9599fc6b504155bc5f59c546e86a
001cf673e2d1df49f03f2317186b1e138262d652d03ce7f2e226f683721fe86c
0026eef688b0780238feafa3fb2a2871cca4cd4e5111667c73f23a42e5b7e68c
002b9311b8eeab02ce0feb1a5b4e00015487155009a7a87dd02035cb2d699a5d
002e9a8fe3714ba07d1b4b983195f60a1a67828ddcbdaf1f44da723670d5f9ca
0030d1c8372ee0d818f34db9b19660c095dcc71a6aeeab5be3178e64179b0b92

jeff@ub2204:~/myprojects/scanner-cache$ ./scancache -action search -key 000a9082db27daf54d52ea0969296174813b7584542ee3d7a256df977a6445dc
Search by key '000a9082db27daf54d52ea0969296174813b7584542ee3d7a256df977a6445dc'
✔️ fetch key=000a9082db27daf54d52ea0969296174813b7584542ee3d7a256df977a6445dc, RefCount=1, value_length (bytes)=760, time=403.087µs

```

</details>
