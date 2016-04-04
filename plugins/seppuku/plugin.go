package seppuku

import (
	"log"
	"os"
	"time"

	"github.com/netflix/hal-9001/hal"
)

func Register() {
	p := hal.Plugin{
		Name:  "seppuku",
		Func:  seppuku,
		Regex: "^!seppuku",
	}
	p.Register()
}

func seppuku(evt hal.Evt) {
	evt.Reply("sayonara")
	time.Sleep(2 * time.Second)
	log.Printf("exiting due to !sayonara command from %s in %s/%s", evt.User, evt.BrokerName(), evt.Room)
	os.Exit(1337)
}
