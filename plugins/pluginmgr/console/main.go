package main

// usage: go run main.go

import (
	"log"
	"net/http"

	"github.com/netflix/hal-9001/brokers/console"
	"github.com/netflix/hal-9001/hal"
	"github.com/netflix/hal-9001/pluginmgr"
	"github.com/netflix/hal-9001/uptime"
)

func main() {
	bconf := console.Config{}
	broker := bconf.NewBroker("console")

	gb := hal.GetGenericBroker()

	// register the uptime plugin but don't start it (so it can be attached at runtime)
	uptime.Register(gb)

	// register the pluginmgr and start it
	pluginmgr.Register(gb)

	// get the plugin's handle back so we can configure it manually
	pr := hal.PluginRegistry()
	mgr := pr.GetPlugin("pluginmgr")
	inst := mgr.Inst(broker.Channel)
	inst.Regex = "^!plugin"
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
