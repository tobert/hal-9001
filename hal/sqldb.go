package hal

import (
	"database/sql"
	"log"
	"strings"
	"sync"

	_ "github.com/go-sql-driver/mysql"
)

var sqldbSingleton *sql.DB
var initSqlDbOnce sync.Once
var sqlInitCache map[string]struct{}

const SECRETS_KEY_DSN = "hal.dsn"

// DB returns the database singleton.
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

// ForceSqlDBHandle can be used to forcibly replace the DB handle with another
// one, e.g. go-sqlmock. This is mainly here for tests, but it's also useful for
// things like examples/repl to operate with no database.
func ForceSqlDBHandle(db *sql.DB) {
	// trigger the sync.Once so the init code doesn't fire
	initSqlDbOnce.Do(func() {})
	sqldbSingleton = db
}

// SqlInit executes the provided SQL once per runtime.
// SqlInit does not care what's in the sql statements. It does not
// interpret the SQL. Its only job is to execute the provided string.
// Execution is not tracked across restarts so statements still need
// to use CREATE TABLE IF NOT EXISTS or other methods of achieving
// idempotent execution. Errors are returned unmodified, including
// primary key violations, so you may ignore them as needed.
func SqlInit(sql_txt string) error {
	db := SqlDB()

	// avoid a database round-trip by checking an in-memory cache
	// fall through and hit the DB on cold cache
	if _, exists := sqlInitCache[sql_txt]; exists {
		return nil
	}

	// execute the statement
	_, err := db.Exec(sql_txt)
	if err != nil {
		log.Printf("SqlInit() failed on statement '%s':\n%s", sql_txt, err)
		return err
	}

	sqlInitCache[sql_txt] = struct{}{}

	return nil
}
