// Package pluginmgr is a plugin manager for hal that allows users to
// manage plugins from inside chat or over REST.
package pluginmgr

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
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/codegangsta/cli"
	"github.com/netflix/hal-9001/hal"
)

// NAME of the plugin
const NAME = "pluginmgr"

// HELP text
const HELP = `
Examples:
!plugin list
!plugin instances
!plugin save
!plugin attach <plugin> --room <room>
!plugin attach --regex ^!foo <plugin> <room>
!plugin detach <plugin> <room>
!plugin group list
!plugin group add <group_name> <plugin_name>
!plugin group del <group_name> <plugin_name>

e.g.
!plugin attach uptime --room CORE
!plugin detach uptime --room CORE
!plugin save
`

const PluginGroupTable = `
CREATE TABLE IF NOT EXISTS plugin_groups (
    group_name  VARCHAR(191),
    plugin_name VARCHAR(191),
    ts          TIMESTAMP,
    PRIMARY KEY(group_name, plugin_name)
)`

type PluginGroupRow struct {
	Group     string    `json:"group"`
	Plugin    string    `json:"plugin"`
	Timestamp time.Time `json:"timestamp"`
}

type PluginGroup []*PluginGroupRow

// Register makes this plugin available to the system.
func Register() {
	plugin := hal.Plugin{
		Name:  NAME,
		Func:  pluginmgr,
		Regex: "^!plugin",
	}

	plugin.Register()

	hal.SqlInit(PluginGroupTable)

	http.HandleFunc("/v1/plugins", httpPlugins)
}

func pluginmgr(evt hal.Evt) {
	// expose plugin names as subcommands so users can do
	// !plugin attach uptime --regex ^!up --room CORE
	attachCmds := make([]cli.Command, 0)
	detachCmds := make([]cli.Command, 0)

	pr := hal.PluginRegistry()

	for _, p := range pr.PluginList() {
		var name, room, regex string
		name = p.Name

		attachCmd := cli.Command{
			Name:  name,
			Usage: fmt.Sprintf("Attach the %s plugin.", p.Name),
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "regex",
					Value:       p.Regex,
					Destination: &regex,
					Usage:       "set a regex filter to select messages to send the plugin, overriding the plugin default",
				},
				cli.StringFlag{
					Name:        "room",
					Value:       evt.RoomId, // default to the room where the command originated
					Destination: &room,
					Usage:       "the room to attach the plugin to",
				},
			},
			Action: func(c *cli.Context) {
				attachPlugin(c, &evt, room, name, regex)
			},
		}

		attachCmds = append(attachCmds, attachCmd)

		detachCmd := cli.Command{
			Name:  name,
			Usage: fmt.Sprintf("Attach the %s plugin.", p.Name),
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "room",
					Value:       evt.RoomId, // default to the room where the command originated
					Destination: &room,      // should be safe to use this again...
					Usage:       "the room to detach from",
				},
			},
			Action: func(c *cli.Context) {
				detachPlugin(c, &evt, room, name)
			},
		}

		detachCmds = append(detachCmds, detachCmd)
	}

	// have cli write output to a buffer instead of stdio
	outbuf := bytes.NewBuffer([]byte{})

	app := cli.NewApp()
	app.Name = NAME
	app.HelpName = NAME
	app.Usage = "manage plugin instances"
	app.Writer = outbuf
	app.Commands = []cli.Command{
		{
			Name:   "list",
			Usage:  "list the available plugins",
			Action: func(c *cli.Context) { listPlugins(c, &evt) },
		},
		{
			Name:   "instances",
			Usage:  "list the currently attached and running plugins",
			Action: func(c *cli.Context) { listInstances(c, &evt) },
		},
		{
			Name:   "save",
			Usage:  "save the runtime plugin configuration",
			Action: func(c *cli.Context) { savePlugins(c, &evt) },
		},
		{
			Name:        "attach",
			Usage:       "attach a plugin to a room (creates an instance)",
			Subcommands: attachCmds, // composed above
		},
		// for now, plugins are restricted to one instance per room to avoid having to
		// generate and manage some kind of ID, which will probably get added later
		{
			Name:        "detach",
			Usage:       "detach a plugin from a room",
			Subcommands: detachCmds,
		},
		{
			Name:  "group",
			Usage: "manage plugin groups",
			Subcommands: []cli.Command{
				{
					Name:   "list",
					Usage:  "list",
					Action: func(c *cli.Context) { listGroupPlugin(c, &evt) },
				},
				{
					Name:   "add",
					Usage:  "add <group_name> <plugin_name>",
					Action: func(c *cli.Context) { addGroupPlugin(c, &evt) },
				},
				{
					Name:   "del",
					Usage:  "del <group_name> <plugin_name>",
					Action: func(c *cli.Context) { delGroupPlugin(c, &evt) },
				},
			},
		},
	}

	err := app.Run(evt.BodyAsArgv())
	if err != nil {
		log.Fatalf("Command parsing failed: %s", err)
	}

	evt.Reply(outbuf.String())
}

func listPlugins(c *cli.Context, evt *hal.Evt) {
	hdr := []string{"Plugin Name", "Default RE", "Status"}
	rows := [][]string{}
	pr := hal.PluginRegistry()

	for _, p := range pr.ActivePluginList() {
		row := []string{p.Name, p.Regex, "active"}
		rows = append(rows, row)
	}

	for _, p := range pr.InactivePluginList() {
		row := []string{p.Name, p.Regex, "inactive"}
		rows = append(rows, row)
	}

	evt.ReplyTable(hdr, rows)
}

func listInstances(c *cli.Context, evt *hal.Evt) {
	hdr := []string{"Plugin Name", "Broker", "Room", "RE"}
	rows := [][]string{}
	pr := hal.PluginRegistry()

	for _, inst := range pr.InstanceList() {
		row := []string{
			inst.Plugin.Name,
			inst.Broker.Name(),
			inst.RoomId,
			inst.Regex,
		}
		rows = append(rows, row)
	}

	evt.ReplyTable(hdr, rows)
}

func savePlugins(c *cli.Context, evt *hal.Evt) {
	pr := hal.PluginRegistry()

	err := pr.SaveInstances()
	if err != nil {
		evt.Replyf("Error while saving plugin config: %s", err)
	} else {
		evt.Reply("Plugin configuration saved.")
	}
}

func roomToId(evt *hal.Evt, room string) string {
	// the user may have provided --room with a room name
	// try to resolve a roomId with the broker, falling back to the name
	if evt.Broker != nil {
		roomId := evt.Broker.RoomNameToId(room)
		if roomId == "" {
			return room
		}
	}

	return room
}

func attachPlugin(c *cli.Context, evt *hal.Evt, room, pluginName, regex string) {
	pr := hal.PluginRegistry()
	plugin, err := pr.GetPlugin(pluginName)
	if err != nil {
		evt.Replyf("No such plugin: '%s'", plugin)
		return
	}

	roomId := roomToId(evt, room)
	inst := plugin.Instance(roomId, evt.Broker)
	inst.RoomId = roomId
	inst.Regex = regex
	err = inst.Register()
	if err != nil {
		evt.Replyf("Failed to launch plugin '%s' in room id '%s': %s", plugin, roomId, err)

	} else {
		evt.Replyf("Launched an instance of plugin: '%s' in room id '%s'", plugin, roomId)
	}
}

func detachPlugin(c *cli.Context, evt *hal.Evt, room, plugin string) {
	pr := hal.PluginRegistry()
	roomId := roomToId(evt, room)
	instances := pr.FindInstances(roomId, evt.BrokerName(), plugin)

	// there should be only one, for now just log if that is not the case
	if len(instances) > 1 {
		log.Printf("FindInstances(%q, %q) returned %d instances. Expected 0 or 1.",
			room, plugin, len(instances))
	}

	for _, inst := range instances {
		inst.Unregister()
		evt.Replyf("%q/%q unregistered", room, plugin)
	}
}

func GetPluginGroup(group string) (PluginGroup, error) {
	out := make(PluginGroup, 0)
	sql := `SELECT group_name, plugin_name FROM plugin_groups`
	params := []interface{}{}

	if group != "" {
		sql = sql + " WHERE group_name=?"
		params = []interface{}{&group}
	}

	db := hal.SqlDB()
	rows, err := db.Query(sql, params...)
	if err != nil {
		return out, err
	}
	defer rows.Close()

	for rows.Next() {
		pgr := PluginGroupRow{}

		// TODO: add timestamps back after making some helpers for time conversion
		// (code that was here didn't handle NULL)
		err := rows.Scan(&pgr.Group, &pgr.Plugin)
		if err != nil {
			log.Printf("PluginGroup row iteration failed: %s\n", err)
			break
		}

		out = append(out, &pgr)
	}

	return out, nil
}

func (pgr *PluginGroupRow) Save() error {
	sql := `INSERT INTO plugin_groups
	        (group_name, plugin_name, ts) VALUES (?, ?, ?)`

	db := hal.SqlDB()
	_, err := db.Exec(sql, &pgr.Group, &pgr.Plugin, &pgr.Timestamp)
	return err
}

func (pgr *PluginGroupRow) Delete() error {
	sql := `DELETE FROM plugin_groups WHERE group_name=? AND plugin_name=?`

	db := hal.SqlDB()
	_, err := db.Exec(sql, &pgr.Group, &pgr.Plugin)
	return err
}

func listGroupPlugin(c *cli.Context, evt *hal.Evt) {
	pgs, err := GetPluginGroup("")
	if err != nil {
		evt.Replyf("Could not fetch plugin group list: %s", err)
		return
	}

	tbl := make([][]string, len(pgs))
	for i, pgr := range pgs {
		tbl[i] = []string{pgr.Group, pgr.Plugin}
	}

	evt.ReplyTable([]string{"Group Name", "Plugin Name"}, tbl)
}

func addGroupPlugin(c *cli.Context, evt *hal.Evt) {
	args := c.Args()
	if len(args) != 2 {
		evt.Replyf("group add requires 2 arguments, only %d were provided, <group_name> <plugin_name>", len(args))
		return
	}

	pr := hal.PluginRegistry()
	// make sure the plugin name is valid
	plugin, err := pr.GetPlugin(args[1])
	if err != nil {
		evt.Error(err)
		return
	}

	// no checking for group other than "can it be inserted as a string"
	pgr := PluginGroupRow{
		Group:     args[0],
		Plugin:    plugin.Name,
		Timestamp: time.Now(),
	}

	err = pgr.Save()
	if err != nil {
		evt.Replyf("failed to add %q to group %q: %s", pgr.Plugin, pgr.Group, err)
	} else {
		evt.Replyf("added %q to group %q", pgr.Plugin, pgr.Group)
	}
}

func delGroupPlugin(c *cli.Context, evt *hal.Evt) {
	args := c.Args()
	if len(args) != 2 {
		evt.Replyf("group add requires 2 arguments, only %d were provided, <group_name> <plugin_name>", len(args))
		return
	}

	pgr := PluginGroupRow{Group: args[0], Plugin: args[1]}
	err := pgr.Delete()
	if err != nil {
		evt.Replyf("failed to delete %q from group %q: %s", pgr.Plugin, pgr.Group, err)
	} else {
		evt.Replyf("deleted %q from group %q", pgr.Plugin, pgr.Group)
	}
}

func httpPlugins(w http.ResponseWriter, r *http.Request) {
	pr := hal.PluginRegistry()
	plugins := pr.PluginList()
	js, err := json.Marshal(plugins)
	if err != nil {
		log.Fatalf("Failed to marshal plugin list to JSON: %s", err)
	}
	w.Write(js)
}
