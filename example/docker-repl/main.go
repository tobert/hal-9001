package main

import (
	"github.com/chzyer/readline"

	"github.com/netflix/hal-9001/brokers/console"
	"github.com/netflix/hal-9001/hal"
	"github.com/netflix/hal-9001/plugins/docker"
	"github.com/netflix/hal-9001/plugins/pluginmgr"
	"github.com/netflix/hal-9001/plugins/prefmgr"
)

// a simple bot that only implements generic plugins on a repl
// possibly a basis for a command-line client for Slack, etc....

func main() {
	rl, err := readline.New("hal> ")
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	bconf := console.Config{}
	broker := bconf.NewBroker("cli")

	docker.Register()
	pluginmgr.Register()
	prefmgr.Register()

	pr := hal.PluginRegistry()
	pr.GetPlugin("pluginmgr").Instance(broker.Room, broker).Register()
	pr.GetPlugin("prefmgr").Instance(broker.Room, broker).Register()
	pr.GetPlugin("docker").Instance(broker.Room, broker).Register()

	router := hal.Router()
	router.AddBroker(broker)
	go router.Route()

	for {
		line, err := rl.Readline()
		if err != nil {
			break
		}

		broker.Line(line)
	}
}
