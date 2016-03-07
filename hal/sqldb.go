package hal

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"log"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var sqldbSingleton *sql.DB
var initSqlDbOnce sync.Once
var sqlInitCache map[string]struct{}

const SQL_INIT_TABLE = `
CREATE TABLE IF NOT EXISTS sql_init (
	sql_id  VARCHAR(64) PRIMARY KEY,
	sql_txt TEXT NOT NULL,
	ts      TIMESTAMP
)`

const SECRETS_KEY_DSN = "hal.dsn"

// DB returns the database context singleton.
func SqlDB() *sql.DB {
	initSqlDbOnce.Do(func() {
		secrets := Secrets()
		dsn := secrets.Get(SECRETS_KEY_DSN)
		if dsn == "" {
			panic("Startup error: SetSqlDB(dsn) must come before any calls to hal.SqlDB()")
		}

		var err error
		sqldbSingleton, err = sql.Open("mysql", strings.TrimSpace(dsn))
		if err != nil {
			log.Fatalf("Could not connect to database: %s\n", err)
		}

		err = sqldbSingleton.Ping()
		if err != nil {
			log.Fatalf("Pinging database failed: %s\n", err)
		}

		sqlInitCache = make(map[string]struct{})
	})

	return sqldbSingleton
}

// SqlInit executes the provided SQL once. It uses a crude tracking
// table to avoid applying updates multiple times, including across
// restarts. It's crude because there are potential race conditions
// around applying the schema updates + crashes, but they should be
// incredibly rare and fixable by hand.
// SqlInit does not care what's in the sql statements. It does not
// interpret the SQL. Its only job is to execute the provided string.
// If anything goes wrong, it will crash the program and print the error.
func SqlInit(sql_txt string) {
	db := SqlDB()

	// avoid a database round-trip by checking an in-memory cache
	// intentionally fall through and hit the DB on cold cache
	if _, exists := sqlInitCache[sql_txt]; exists {
		log.Printf("Table is already present.")
		return
	}

	sqlInitCache[sql_txt] = struct{}{}

	// make sure the table exists - this will end up executing every time this
	// function is called but it won't hurt anything so keep it simple
	_, err := db.Exec(SQL_INIT_TABLE)
	if err != nil {
		log.Fatalf("SqlInit() failed to create its tracking table: %s", err)
	}

	// statements are identified by their sha256 hash based on the exact
	// string provided. The hash id is quite a bit easier to work with
	// in other database tools than a query looking for a query...
	hash := sha256.Sum256([]byte(sql_txt))
	// use the hex version because mysql BINARY isn't supported in Go (yet?)
	sql_id := hex.EncodeToString(hash[0:])

	// see if this statement has already been executed
	var count int
	q := db.QueryRow("SELECT COUNT(sql_id) FROM sql_init WHERE sql_id=?", sql_id)
	err = q.Scan(&count)
	if err != nil {
		log.Fatalf("SqlInit() failed while checking for previous statements: %s", err)
	}

	// statement already executed, move onto the next one
	if count > 0 {
		return
	}

	// execute the statement
	_, err = db.Exec(sql_txt)
	if err != nil {
		log.Fatalf("SqlInit() failed on statement '%s':\n%s", sql_txt, err)
	}

	// record that it was completed
	now := time.Now()
	_, err = db.Exec("INSERT INTO sql_init (sql_id, sql_txt, ts) VALUES (?, ?, ?)", sql_id, sql_txt, now)
	if err != nil {
		log.Fatalf("SqlInit() failed to record execution of statement '%s':\n%s", sql_txt, err)
	}
}
