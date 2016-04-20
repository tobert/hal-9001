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
	"bytes"
	"fmt"
	"net/http"
	"strconv"

	"github.com/codegangsta/cli"
	"github.com/netflix/hal-9001/hal"
)

const NAME = "prefmgr"

const HELP = `Listing keys with no filter will list all keys visible to the active user and room.

!prefs list --key KEY
!prefs list --user USER --room CHANNEL --plugin PLUGIN --key KEY --def DEFAULT
`

func Register() {
	plugin := hal.Plugin{
		Name:  NAME,
		Func:  prefmgr,
		Regex: "^!prefs",
	}
	plugin.Register()

	http.HandleFunc("/v1/prefs", httpPrefs)
}

func prefmgr(evt hal.Evt) {
	flags := hal.Pref{}

	valFlag := cli.StringFlag{
		Name:        "value",
		Destination: &flags.Value,
		Value:       "",
		Usage:       "the value",
	}

	keyFlag := cli.StringFlag{
		Name:        "key",
		Destination: &flags.Key,
		Value:       "",
		Usage:       "the preference key to match",
	}

	pluginFlag := cli.StringFlag{
		Name:        "plugin",
		Destination: &flags.Plugin,
		Value:       "",
		Usage:       "select only prefs for the provided plugin",
	}

	brokerFlag := cli.StringFlag{
		Name:        "broker",
		Destination: &flags.Broker,
		Value:       "",
		Usage:       "select only prefs for the provided broker",
	}

	roomFlag := cli.StringFlag{
		Name:        "room",
		Destination: &flags.Room,
		Value:       "",
		Usage:       "select only prefs for the provided room",
	}

	userFlag := cli.StringFlag{
		Name:        "user",
		Destination: &flags.User,
		Value:       "",
		Usage:       "select only prefs for the provided user",
	}

	outbuf := bytes.NewBuffer([]byte{})

	app := cli.NewApp()
	app.Name = NAME
	app.HelpName = NAME
	app.Usage = "manage preferences"
	app.Writer = outbuf
	app.Commands = []cli.Command{
		{
			Name:  "list",
			Usage: "list available preferences",
			Flags: []cli.Flag{keyFlag, pluginFlag, brokerFlag, roomFlag, userFlag},
			Action: func(ctx *cli.Context) {
				cliList(ctx, evt, flags)
			},
		},
		{
			Name:  "set",
			Usage: "set a preference key",
			Flags: []cli.Flag{keyFlag, pluginFlag, brokerFlag, roomFlag, userFlag, valFlag},
			Action: func(ctx *cli.Context) {
				cliSet(ctx, evt, flags)
			},
		},
		{
			Name:  "rm",
			Usage: "delete a preference key by id. e.g. !prefs rm 1",
			Action: func(ctx *cli.Context) {
				cliRm(ctx, evt)
			},
		},
	}

	err := app.Run(evt.BodyAsArgv())
	if err != nil {
		evt.Reply(fmt.Sprintf("Unable to parse your command, '%s': %s", evt.Body, err))
	}

	evt.Reply(outbuf.String())
}

func cliList(ctx *cli.Context, evt hal.Evt, opts hal.Pref) {
	prefs := opts.Find()
	data := prefs.Table()
	evt.ReplyTable(data[0], data[1:])
}

func cliSet(ctx *cli.Context, evt hal.Evt, opts hal.Pref) {
	if opts.Key == "" {
		evt.Reply("--key is required to set prefs")
		return
	}

	if opts.Value == "" {
		evt.Reply("--value is required to set prefs")
		return
	}

	// shorthand for "current room/user/broker" !pref set --user * --room * --broker *
	if opts.User == "*" {
		opts.User = evt.UserId
	}

	if opts.Room == "*" {
		opts.Room = evt.RoomId
	}

	if opts.Broker == "*" {
		opts.Broker = evt.BrokerName()
	}

	fmt.Printf("Setting pref: %q\n", opts.String())
	err := opts.Set()
	if err != nil {
		evt.Replyf("Failed to set pref: %q", err)
	} else {
		data := opts.GetPrefs().Table()
		evt.ReplyTable(data[0], data[1:])
	}
}

func cliRm(ctx *cli.Context, evt hal.Evt) {
	args := ctx.Args()
	if len(args) != 1 {
		evt.Reply("!prefs rm requires exactly one argument.")
		return
	}

	id, err := strconv.Atoi(args[0])
	if err != nil {
		evt.Replyf("Failed to delete pref: %q does not seem to be an integer. %s", args[0], err)
		return
	}

	err = hal.RmPrefId(id)
	if err != nil {
		evt.Replyf("Failed to delete pref with id %d: %s", id, err)
	} else {
		evt.Replyf("Deleted pref id %d.", id)
	}
}

func httpPrefs(w http.ResponseWriter, r *http.Request) {
}
