// Package pluginmgr is a plugin manager for hal that allows users to
// manage plugins from inside chat or over REST.
package pluginmgr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

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

e.g.
!plugin attach uptime --room CORE
!plugin detach uptime --room CORE
!plugin save
`

// Register makes this plugin available to the system.
func Register() {
	plugin := hal.Plugin{
		Name:  NAME,
		Func:  pluginmgr,
		Regex: "^!plugin",
	}

	plugin.Register()

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
	plugin := pr.GetPlugin(pluginName)
	if plugin == nil {
		evt.Replyf("No such plugin: '%s'", plugin)
		return
	}

	roomId := roomToId(evt, room)
	inst := plugin.Instance(roomId, evt.Broker)
	inst.RoomId = roomId
	inst.Regex = regex
	err := inst.Register()
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

func httpPlugins(w http.ResponseWriter, r *http.Request) {
	pr := hal.PluginRegistry()
	plugins := pr.PluginList()
	js, err := json.Marshal(plugins)
	if err != nil {
		log.Fatalf("Failed to marshal plugin list to JSON: %s", err)
	}
	w.Write(js)
}
