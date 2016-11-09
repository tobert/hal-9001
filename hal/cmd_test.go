package hal

import (
	"strings"
	"testing"
)

func TestCmd(t *testing.T) {
	assertError := func(err error) {
		if err != nil {
			t.Error(err)
			t.Fail()
		}
	}

	// example 1 - smoke test
	oc := NewCmd("oncall", true).
		SetUsage("search Pagerduty escalation policies for a string")
	oc.AddSubCmd("cache-status")
	oc.AddSubCmd("cache-interval").AddIdxParam(0, "interval", true)
	//oc.AddCmd("*"), // everything else is a search string

	oc.GetSubCmd("cache-status").SetUsage("check the status of the background caching job")
	oc.GetSubCmd("cache-interval").SetUsage("set the background caching job interval")
	//oc.GetSubCmd("*").SetUsage("create a mark in time with an (optional) text note")

	// evt.BodyAsArgv()
	var res *CmdInst
	var err error
	// make sure a command with no args doesn't blow up
	res, err = oc.Process([]string{"!oncall"})
	assertError(err)

	res, err = oc.Process([]string{"!oncall", "help"})
	assertError(err)

	// TODO: add help functionality and auto-wire it
	res, err = oc.Process([]string{"!oncall", "h"})
	assertError(err)

	res, err = oc.Process([]string{"!oncall", "sre"})
	assertError(err)
	if len(res.Remainder()) != 1 || res.Remainder()[0] != "sre" {
		t.Fail()
	}

	res, err = oc.Process([]string{"!oncall", "cache-status"})
	assertError(err)
	if res.SubCmdToken() != "cache-status" {
		t.Fail()
	}

	res, err = oc.Process([]string{"!oncall", "cache-interval"})
	assertError(err)
	if res.SubCmdToken() != "cache-interval" {
		t.Fail()
	}

	// example 2
	// Alias: requiring explicit aliases instead of guessing seems right
	pc := NewCmd("prefs", true)
	pc.AddSubCmd("set").
		SetUsage("set a pref").
		SubCmd().AddKVParam("key", true).AddAlias("k").SetUsage("ohai!").
		SubCmd().AddKVParam("value", true).AddAlias("v").
		SubCmd().AddKVParam("room", false).AddAlias("r").
		SubCmd().AddKVParam("user", false).AddAlias("u").
		SubCmd().AddKVParam("broker", false).AddAlias("b")

	pc.AddSubCmd("get").
		SubCmd().AddKVParam("key", true).AddAlias("k").
		SubCmd().AddKVParam("value", true).AddAlias("v").
		SubCmd().AddKVParam("room", false).AddAlias("r").
		SubCmd().AddKVParam("user", false).AddAlias("u").
		SubCmd().AddKVParam("broker", false).AddAlias("b")

	pc.AddSubCmd("rm").AddIdxParam(0, "id", true)

	argv2 := strings.Split("prefs set --room * --user foo --broker console --key ohai --value nevermind", " ")
	res, err = pc.Process(argv2)
	assertError(err)

	if len(res.Remainder()) != 0 {
		t.Error("There should not be any remainder")
	}
	if res.SubCmdToken() != "set" {
		t.Errorf("wrong subcommand. Expected %q, got %q", "set", res.SubCmdToken())
	}
	if res.SubCmdInst() == nil {
		t.Error("result.SubCmdInst is nil when it should be an instance for 'set'")
		t.FailNow()
	}
	subcmd := res.SubCmdInst()
	if subcmd.GetKVParamInst("room").MustString() != "*" {
		t.Errorf("wrong room, expected *, got %q", subcmd.GetKVParamInst("room").MustString())
	}
	if subcmd.GetKVParamInst("key").MustString() != "ohai" {
		t.Errorf("wrong key, expected 'ohai', got %q", subcmd.GetKVParamInst("key").MustString())
	}
	if subcmd.GetKVParamInst("value").MustString() != "nevermind" {
		t.Errorf("wrong value, expected 'nevermind', got %q", subcmd.GetKVParamInst("value").MustString())
	}
	// check that defaults are working
	dval := "1234"
	rds := subcmd.GetKVParamInst("room").DefString(dval)
	if rds != dval {
		t.Errorf("DefString returned %q, expected %q", rds, dval)
	}
	irds := subcmd.GetKVParamInst("room").DefInt(999)
	if irds != 999 {
		t.Errorf("DefString returned %d, expected 999", irds)
	}

	// again with out-of-order parameters
	argv3 := strings.Split("prefs --user bob --key testing get --value lol", " ")
	res, err = pc.Process(argv3)
	assertError(err)
	if len(res.Remainder()) != 0 {
		t.Error("There should not be any remainder")
	}
	if res.SubCmdToken() != "get" {
		t.Errorf("wrong subcommand. Expected 'get', got %q", res.SubCmdToken())
	}
	if res.SubCmdInst() == nil {
		t.Error("result.SubCmdInst is nil when it should be an instance for 'get'")
		t.FailNow()
	}
	subcmd = res.SubCmdInst()
	kvpi := subcmd.GetKVParamInst("key")
	if kvpi == nil {
		t.Error("BUG: subcmd.GetKVParamInst('key') returned nil")
		t.FailNow()
	}
	if kvpi.MustString() != "testing" {
		t.Errorf("wrong key, expected 'testing', got %q", subcmd.GetKVParamInst("key").MustString())
	}

	argv4 := []string{"!prefs", "rm", "4"}
	res, err = pc.Process(argv4)
	assertError(err)
	if res.SubCmdToken() != "rm" {
		t.Errorf("Expected rm, got %q", res.SubCmdToken())
	}
	pp := res.SubCmdInst().GetIdxParamInst(0)
	if pp.Value() != "4" {
		t.Errorf("wrong value from positional parameter. got %d, expected 4", pp.idx)
	}

	/*
		// make sure it doesn't blow up on invalid subcmd
		argv5 := []string{"!prefs", "asdfasdfasdfasdf", "asdf"}
		res = pc.Process(argv5)
		// at this point res.SubCmdInst is nil ... *sigh*
		res.SubCmdInst.GetPParamInst(0)
	*/
}
