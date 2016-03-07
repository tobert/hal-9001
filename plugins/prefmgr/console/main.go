package main

// usage: go run main.go

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/netflix/hal-9001/brokers/console"
	"github.com/netflix/hal-9001/hal"
	"github.com/netflix/hal-9001/prefmgr"
)

func main() {
	// start up the SQL database first - it's required
	// TODO: update to use secrets API
	dbuser := os.Getenv("SQLDB_USER")
	dbpass := os.Getenv("SQLDB_PASS")
	dbname := os.Getenv("SQLDB_DBNAME")
	dsn := fmt.Sprintf("%s:%s@/%s", dbuser, dbpass, dbname)

	hal.SetSqlDSN(dsn)
	hal.SqlDB() // fires up the connection

	bconf := console.Config{}
	broker := bconf.NewBroker("console")

	gb := hal.GetGenericBroker()

	// register the pluginmgr and start it
	prefmgr.Register(gb)

	// get the plugin's handle back so we can configure it manually
	pr := hal.PluginRegistry()
	mgr := pr.GetPlugin("prefmgr")
	inst := mgr.Inst(broker.Channel)
	inst.Register()

	// attach the console broker
	hal.Router().AddBroker(broker)

	// start up the message routing goroutine
	go hal.Router().Route()

	err := http.ListenAndServe(":42001", nil)
	if err != nil {
		log.Fatalf("Could not listen on port 42000: %s\n", err)
	}
}
