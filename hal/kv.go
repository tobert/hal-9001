package hal

/*
 * Copyright 2016 Albert P. Tobey <atobey@netflix.com>
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import (
	dbsql "database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/juju/errors"
)

const KVTable = `
CREATE TABLE IF NOT EXISTS kv (
	 pkey    VARCHAR(191) NOT NULL,
	 value   MEDIUMTEXT,
	 expires DATETIME,
	 ttl     INT NOT NULL DEFAULT 0, -- ttl 0 is forever
	 ts      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	 PRIMARY KEY(pkey)
)`

type kvRecord struct {
	key     string
	value   string
	expires time.Time     // when the key expires
	ttl     time.Duration // the desired ttl
	ttlSecs int64         // raw value from the db
}

type KVExpiredTtlError struct {
	Key     string
	Ttl     time.Duration
	Expires time.Time
}

var kvLateInitOnce sync.Once
var kvCache map[string]*kvRecord
var kvMut sync.Mutex

func init() {
	kvCache = make(map[string]*kvRecord)
}

func kvLazyInit() {
	kvLateInitOnce.Do(func() {
		SqlInit(KVTable)
		go kvCleanup()
	})
}

func kvCleanup() {
	c := time.Tick(time.Minute)

	for now := range c {
		log.Printf("Cleaning up ttl keys")

		kvMut.Lock()
		defer kvMut.Unlock()

		db := SqlDB()
		_, err := db.Exec("DELETE FROM kv WHERE expires < NOW()")
		if err != nil {
			log.Printf("DELETE of expired keys from the DB failed: %s", err)
		}

		// clean the in-memory cache
		for key, kv := range kvCache {
			if now.After(kv.expires) {
				log.Printf("Deleting %q from the kvCache. It expired at %s", key, kv.expires)
				delete(kvCache, key)
			}
		}
	}
}

// ExistsKV checks to see if a key exists in the kv. False if any errors are
// encountered.
func ExistsKV(key string) bool {
	_, err := GetKV(key)
	if err != nil {
		return false
	}

	return true
}

// NOTE: this will probably change to an ok,value style
func GetKV(key string) (value string, err error) {
	kvLazyInit()
	db := SqlDB()
	now := time.Now()

	kvMut.Lock()
	defer kvMut.Unlock()

	// check the cache and return immediately if it's present and valid
	if cached, exists := kvCache[key]; exists {
		if now.After(cached.expires) {
			delete(kvCache, key)
		} else {
			return cached.value, nil
		}
	}

	kv := kvRecord{key: key}

	var expireTs int64
	sql := "SELECT value,ttl,UNIX_TIMESTAMP(expires) FROM kv WHERE pkey=?"
	err = db.QueryRow(sql, key).Scan(&kv.value, &kv.ttlSecs, &expireTs)
	if err == dbsql.ErrNoRows {
		// TODO: might just want to swallow errors and return two-val exists,value instead
		return "", errors.NewNotFound(err, sql)
	} else if err != nil {
		return "", errors.Annotate(err, "GetKV SQL query failed")
	}

	kv.expires = time.Unix(expireTs, 0)
	kv.ttl = time.Second * time.Duration(kv.ttlSecs)

	// 0 seconds means no ttl
	if kv.ttlSecs == 0 {
		return kv.value, nil
	}

	// check the ttl and return empty string + an error if it's expired
	if now.After(kv.expires) {
		delete(kvCache, key)
		return "", kv.NewKVExpiredTtlError()
	}

	return kv.value, nil
}

func SetKV(key, value string, ttl time.Duration) (err error) {
	kvLazyInit()

	kvMut.Lock()
	defer kvMut.Unlock()

	now := time.Now()

	kvCache[key] = &kvRecord{
		key:     key,
		value:   value,
		ttl:     ttl,
		expires: now.Add(ttl),
	}

	db := SqlDB()
	_, err = db.Exec("INSERT INTO kv (pkey,value,expires,ttl) VALUES (?,?,?,?)",
		key, value, now.Add(ttl), int(ttl.Seconds()))

	if err != nil {
		log.Printf("SetKV INSERT failed: %s", err)
	}

	return err
}

func (kv *kvRecord) NewKVExpiredTtlError() KVExpiredTtlError {
	return KVExpiredTtlError{
		Key:     kv.key,
		Ttl:     kv.ttl,
		Expires: kv.expires,
	}
}

func (e KVExpiredTtlError) Error() string {
	return fmt.Sprintf("key %q expired at %s after its ttl of %s ran out",
		e.Key, e.Expires.String(), e.Ttl.String())
}
