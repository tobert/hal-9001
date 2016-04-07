package main

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
	"fmt"
	"time"

	"github.com/chzyer/readline"

	"gopkg.in/DATA-DOG/go-sqlmock.v1"

	"github.com/netflix/hal-9001/brokers/console"
	"github.com/netflix/hal-9001/hal"
	"github.com/netflix/hal-9001/plugins/pluginmgr"
	"github.com/netflix/hal-9001/plugins/prefmgr"
	"github.com/netflix/hal-9001/plugins/uptime"
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

	// SqlInit calls will still throw errors at startup but
	// it seems the program will continue so this will do for now
	db, _, err := sqlmock.New()
	if err != nil {
		panic(err)
	}
	hal.ForceSqlDBHandle(db)
	defer db.Close()

	pluginmgr.Register()
	prefmgr.Register()
	uptime.Register()

	pr := hal.PluginRegistry()
	pmp, _ := pr.GetPlugin("pluginmgr")
	pmp.Instance(broker.Room, broker).Register()

	router := hal.Router()
	router.AddBroker(broker)
	go router.Route()

	lines := make(chan string, 1)

	go func() {
		for {
			// prevent the prompt from being printed at the wrong time
			// this is awful but good 'nuf for a demo
			time.Sleep(time.Second/4)

			line, err := rl.Readline()
			if err != nil {
				return
			}

			lines <- line
		}
	}()

	for {
		select {
		case line := <-broker.Stdout:
			fmt.Println(line)
		case line := <-lines:
			broker.Stdin <- line
		}
	}
}
