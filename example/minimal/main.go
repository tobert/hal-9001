package main

import (
	"github.com/netflix/hal-9001/brokers/generic"
	"github.com/netflix/hal-9001/hal"
)

// This bot doesn't do anything except set up the generic broker and then
// block forever. The generic broker doesn't produce anything so nothing
// will happen and this is totally useless except to demonstrate the minimum
// amount of hal's API required to start the system.
//
// Most of hal's functionality is optional. It's still built along with the
// rest of hal but is not active unless it's used in main or a plugin.

func main() {
	conf := generic.Config{}
	broker := conf.NewBroker("generic")

	router := hal.Router()
	router.AddBroker(broker)
	router.Route()

	// TODO: maybe add a timer loop to inject some messages and exercise
	// the system a little.
}
