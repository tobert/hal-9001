// prefmgr exposes hal's preferences as a bot command and over REST
package prefmgr

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
	"net/http"

	"github.com/netflix/hal-9001/hal"
)

const NAME = "prefmgr"

const HELP = `Listing keys with no filter will list all keys visible to the active user and room.

!prefs list --key KEY
!prefs list --user USER --room CHANNEL --plugin PLUGIN --key KEY --def DEFAULT
`

var cli *hal.Cmd

func init() {
	cli = &hal.Cmd{
		Token:      "pref",
		Usage:      "Manage hal preferences over chat.",
		MustSubCmd: true,
	}

	keyUsage := "the key name, up to 190 utf8 characters"
	valueUsage := "the value, arbitrary utf8"
	roomUsage := "the chat room id (usually auto-resolved, '*' for 'this room')"
	userUsage := "the user id (usually auto-resolved, '*' for 'executing user')"
	brokerUsage := "the broker name. e.g. 'slack' ('*' for 'this broker')"
	pluginUsage := "the plugin name. e.g. 'archive' ('*' for 'this plugin')"

	cli.AddCmd("set").
		AddUsage("set a preference key/value").
		Cmd().AddParam("key", true).AddAlias("k").AddUsage(keyUsage).
		Cmd().AddParam("value", true).AddAlias("v").AddUsage(valueUsage).
		Cmd().AddParam("room", false).AddAlias("r").AddUsage(roomUsage).
		Cmd().AddParam("user", false).AddAlias("u").AddUsage(userUsage).
		Cmd().AddParam("broker", false).AddAlias("b").AddUsage(brokerUsage).
		Cmd().AddParam("plugin", false).AddAlias("p").AddUsage(pluginUsage)

	cli.AddCmd("list").AddAlias("get").
		AddUsage("retreive preferences, optionally filtered by the provided attributes").
		Cmd().AddParam("key", false).AddAlias("k").AddUsage(keyUsage).
		Cmd().AddParam("value", false).AddAlias("v").AddUsage(valueUsage).
		Cmd().AddParam("room", false).AddAlias("r").AddUsage(roomUsage).
		Cmd().AddParam("user", false).AddAlias("u").AddUsage(userUsage).
		Cmd().AddParam("broker", false).AddAlias("b").AddUsage(brokerUsage).
		Cmd().AddParam("plugin", false).AddAlias("p").AddUsage(pluginUsage)

	cli.AddCmd("rm").
		AddUsage("delete a preference by id").
		AddPParam(0, true).AddUsage("the preference id to delete")
}

func Register() {
	plugin := hal.Plugin{
		Name:  NAME,
		Func:  prefmgr,
		Regex: "^!prefs",
	}
	plugin.Register()

	http.HandleFunc("/v1/prefs", httpPrefs)
}

// prefmgr is called when someone executes !pref in the chat system
func prefmgr(evt hal.Evt) {
	req := cli.Process(evt.BodyAsArgv())

	switch req.SubCmdToken() {
	case "set":
		cliSet(req, &evt)
	case "list":
		cliList(req, &evt)
	case "rm":
		cliRm(req, &evt)
	default:
		evt.Reply(req.RenderUsage("invalid command"))
	}
}

// cmd2pref copies data from the hal.Cmd and hal.Evt into a hal.Pref, resolving
// *'s on the way.
func cmd2pref(req *hal.CmdInst, evt *hal.Evt) (*hal.Pref, error) {
	var out hal.Pref

	for _, pi := range req.ParamInsts {
		var err error

		switch pi.Key {
		case "key":
			out.Key, err = pi.String()
		case "value":
			out.Value, err = pi.String()
		case "room":
			out.Room = pi.DefString(evt.RoomId)
		case "user":
			out.User = pi.DefString(evt.UserId)
		case "broker":
			out.Broker = pi.DefString(evt.BrokerName())
		case "plugin":
			out.Plugin, _ = pi.String()
		}

		// return on the first error
		if err != nil {
			return nil, err
		}
	}

	return &out, nil
}

// cliList implements !pref list
func cliList(req *hal.CmdInst, evt *hal.Evt) {
	opts, err := cmd2pref(req, evt)
	if err != nil {
		panic(err) // TODO: placeholder
	}

	prefs := opts.Find()
	data := prefs.Table()
	evt.ReplyTable(data[0], data[1:])
}

// cliSet implements !pref set
func cliSet(req *hal.CmdInst, evt *hal.Evt) {
	opts, err := cmd2pref(req, evt)
	if err != nil {
		panic(err) // TODO: placeholder
	}

	if opts.Room != "" && !evt.Broker.LooksLikeRoomId(opts.Room) {
		opts.Room = evt.Broker.RoomNameToId(opts.Room)
	}

	if opts.User != "" && !evt.Broker.LooksLikeUserId(opts.User) {
		opts.User = evt.Broker.UserNameToId(opts.User)
	}

	// TODO: check plugin name validity
	// TODO: check broker name validity

	fmt.Printf("Setting pref: %q\n", opts.String())
	err = opts.Set()
	if err != nil {
		evt.Replyf("Failed to set pref: %q", err)
	} else {
		data := opts.GetPrefs().Table()
		evt.ReplyTable(data[0], data[1:])
	}
}

// cliRm implements !pref rm <id>
func cliRm(req *hal.CmdInst, evt *hal.Evt) {
	id, err := req.GetPParamInst(0).Int()
	if err != nil {
		panic(err) // TODO: placeholder
	}

	err = hal.RmPrefId(id)
	if err != nil {
		evt.Replyf("Failed to delete pref with id %d: %s", id, err)
	} else {
		evt.Replyf("Deleted pref id %d.", id)
	}
}

// httpPrefs is the http handler for returning preferences as JSON
func httpPrefs(w http.ResponseWriter, r *http.Request) {
}
