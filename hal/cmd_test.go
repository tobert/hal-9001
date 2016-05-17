package hal

import (
	"strings"
	"testing"
	//"github.com/davecgh/go-spew/spew"
)

func TestCmd(t *testing.T) {
	// example 1
	oc := Cmd{
		Token:      "oncall",
		MustSubCmd: true,
		Usage:      "search Pagerduty escalation policies for a string",
		SubCmds: []*Cmd{
			NewCmd("cache-status"),
			NewCmd("cache-interval").AddPParam(0, true).Cmd(),
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
	var res *CmdInst
	res = oc.Process([]string{"!oncall", "help"})
	// TODO: add help functionality and auto-wire it
	res = oc.Process([]string{"!oncall", "h"})

	res = oc.Process([]string{"!oncall", "sre"})
	if len(res.Remainder) != 1 || res.Remainder[0] != "sre" {
		t.Fail()
	}

	res = oc.Process([]string{"!oncall", "cache-status"})
	if res.SubCmdToken() != "cache-status" {
		t.Fail()
	}

	res = oc.Process([]string{"!oncall", "cache-interval"})
	if res.SubCmdToken() != "cache-interval" {
		t.Fail()
	}

	// example 2
	// Alias: requiring explicit aliases instead of guessing seems right
	pc := NewCmd("prefs")
	pc.AddCmd("set").
		AddUsage("set a pref").
		Cmd().AddParam("key", true).AddAlias("k").AddUsage("ohai!").
		Cmd().AddParam("value", true).AddAlias("v").
		Cmd().AddParam("room", false).AddAlias("r").
		Cmd().AddParam("user", false).AddAlias("u").
		Cmd().AddParam("broker", false).AddAlias("b")

	pc.AddCmd("get").
		Cmd().AddParam("key", true).AddAlias("k").
		Cmd().AddParam("value", true).AddAlias("v").
		Cmd().AddParam("room", false).AddAlias("r").
		Cmd().AddParam("user", false).AddAlias("u").
		Cmd().AddParam("broker", false).AddAlias("b")

	argv2 := strings.Split("prefs set --room * --user foo --broker console --key ohai --value nevermind", " ")
	res = pc.Process(argv2)

	//spew.Dump(res)

	if len(res.Remainder) != 0 {
		t.Error("There should not be any remainder")
	}
	if res.SubCmdToken() != "set" {
		t.Errorf("wrong subcommand. Expected %q, got %q", "set", res.SubCmdToken())
	}
	if res.SubCmdInst == nil {
		t.Error("result.SubCmdInst is nil when it should be an instance for 'set'")
		t.FailNow()
	}
	subcmd := res.SubCmdInst
	if subcmd.GetParamInst("room").MustString() != "*" {
		t.Errorf("wrong room, expected *, got %q", subcmd.GetParamInst("room").MustString())
	}
	if subcmd.GetParamInst("key").MustString() != "ohai" {
		t.Errorf("wrong key, expected 'ohai', got %q", subcmd.GetParamInst("key").MustString())
	}
	if subcmd.GetParamInst("value").MustString() != "nevermind" {
		t.Errorf("wrong value, expected 'nevermind', got %q", subcmd.GetParamInst("value").MustString())
	}

	// again with out-of-order parameters
	argv3 := strings.Split("prefs --user bob --key testing get --value lol", " ")
	res = pc.Process(argv3)
	if len(res.Remainder) != 0 {
		t.Error("There should not be any remainder")
	}
	if res.SubCmdToken() != "get" {
		t.Errorf("wrong subcommand. Expected 'get', got %q", res.SubCmdToken())
	}
	if res.SubCmdInst == nil {
		t.Error("result.SubCmdInst is nil when it should be an instance for 'get'")
		t.FailNow()
	}
	subcmd = res.SubCmdInst
	if subcmd.GetParamInst("key").MustString() != "testing" {
		t.Errorf("wrong key, expected 'testing', got %q", subcmd.GetParamInst("key").MustString())
	}
}
