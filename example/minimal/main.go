package main

import (
	"os"

	"github.com/netflix/hal-9001/brokers/slack"
	"github.com/netflix/hal-9001/hal"
)

// This bot doesn't do anything except log into slack and receive messages.
//
// Most of hal's functionality is optional. It's still built along with the
// rest of hal but is not active unless it's used in main or a plugin.

func main() {
	sconf := slack.Config{
		Token: os.Getenv("SLACK_TOKEN"),
	}
	slk := sconf.NewBroker("slack")

	router := hal.Router()
	router.AddBroker(slk)
	router.Route()
}
