package hal

import (
	"strings"
	"testing"
)

func TestCmd(t *testing.T) {
	// example 1
	oc := Cmd{
		Token:      "oncall",
		MustSubCmd: true,
		Usage:      "search Pagerduty escalation policies for a string",
		SubCmds: []*Cmd{
			NewCmd("cache-status"),
			NewCmd("cache-interval").AddPParam(0, "1h", true),
			NewCmd("*"), // everything else is a search string
		},
	}

	oc.GetSubCmd("cache-status").Usage = "check the status of the background caching job"
	oc.GetSubCmd("cache-interval").Usage = "set the background caching job interval"
	oc.GetSubCmd("*").Usage = "create a mark in time with an (optional) text note"
	// hmm maybe we can abuse varargs a bit without ruining safety....
	// basically achieves a type-safe kwargs...
	// NewCmd("*", Usage{"create a mark in time with an (optional) text note"})

	// evt.BodyAsArgv()
	oc.Process([]string{"!oncall", "help"})
	oc.Process([]string{"!oncall", "h"})
	oc.Process([]string{"!oncall", "sre"})
	oc.Process([]string{"!oncall", "cache-status"})
	oc.Process([]string{"!oncall", "cache-interval"})

	/*
		switch oci.SubCmdToken() {
		case "cache-status":
			cacheStatus(&evt)
		case "cache-interval":
			cacheInterval(&evt, oci)
		case "*":
			search(&evt, oci)
		}
	*/

	// example 2
	// Alias: requiring explicit aliases instead of guessing seems right
	pc := NewCmd("prefs")
	pc.AddCmd("set").
		AddParam("key", "", true).
		AddAlias("key", "k"). // vertically aligned for your viewing pleasure
		AddParam("value", "", true).
		AddAlias("value", "v").
		AddParam("room", "", false).
		AddAlias("room", "r").
		AddUsage("room", "Set the room ID").
		AddParam("user", "", false).
		AddAlias("user", "u").
		AddParam("broker", "", false).
		AddAlias("broker", "b")
	// ^ in an init func, stuff below in the callback

	//cmd := pc.Process(evt.BodyAsArgv())
	argv2 := strings.Split("prefs set --room * --user foo --broker console --key ohai --value nevermind", " ")
	pc.Process(argv2)
	/*
		pref := hal.Pref{
			Key:    cmd.GetParam("key").MustString(),
			Value:  cmd.GetParam("value").MustString(),
			Room:   cmd.GetParam("room").DefString(evt.RoomId),
			User:   cmd.GetParam("user").DefString(evt.UserId),
			Broker: cmd.GetParam("borker").DefString(evt.BrokerName()),
		}

		switch cmd.SubCmdToken() {
		case "set":
			pref.Set()
			evt.Reply("saved!")
		case "get":
			got := pref.Get()
			tbl := hal.Prefs{got}.Table()
			evt.ReplyTable(tbl[0], tbl[1:])
		case "find":
			prefs := pref.Find()
			tbl := prefs.Table()
			evt.ReplyTable(tbl[0], tbl[1:])
		}
	*/

}
