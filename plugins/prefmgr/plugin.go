// prefmgr exposes hal's preferences as a bot command and over REST
package prefmgr

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/codegangsta/cli"
	"github.com/netflix/hal-9001/hal"
)

const NAME = "prefmgr"

const HELP = `Listing keys with no filter will list all keys visible to the active user and channel.

!prefs list --key KEY
!prefs list --user USER --chan CHANNEL --plugin PLUGIN --key KEY --def DEFAULT
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
		Usage:       "the value",
	}

	keyFlag := cli.StringFlag{
		Name:        "key",
		Destination: &flags.Key,
		Usage:       "the preference key to match",
	}

	pluginFlag := cli.StringFlag{
		Name:        "plugin",
		Destination: &flags.Plugin,
		Usage:       "select only prefs for the provided plugin",
	}

	brokerFlag := cli.StringFlag{
		Name:        "broker",
		Destination: &flags.Broker,
		Usage:       "select only prefs for the provided broker",
	}

	channelFlag := cli.StringFlag{
		Name:        "channel",
		Destination: &flags.Room,
		Usage:       "select only prefs for the provided channel",
	}

	userFlag := cli.StringFlag{
		Name:        "user",
		Destination: &flags.User,
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
			Flags: []cli.Flag{keyFlag, pluginFlag, brokerFlag, channelFlag, userFlag},
			Action: func(ctx *cli.Context) {
				cliList(ctx, evt, flags)
			},
		},
		{
			Name:  "get",
			Usage: "get a preference key",
			Flags: []cli.Flag{keyFlag, pluginFlag, brokerFlag, channelFlag, userFlag},
			Action: func(ctx *cli.Context) {
				cliGet(ctx, evt, flags)
			},
		},
		{
			Name:  "set",
			Usage: "set a preference key",
			Flags: []cli.Flag{keyFlag, pluginFlag, brokerFlag, channelFlag, userFlag, valFlag},
			Action: func(ctx *cli.Context) {
				cliSet(ctx, evt, flags)
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
	//text := hal.AsciiTable(data[0], data[1:])
	//evt.Reply(text)
}

func cliGet(ctx *cli.Context, evt hal.Evt, opts hal.Pref) {
	pref := opts.Get()
	// TODO: humanitarian formatting
	evt.Replyf("%v", pref)
}

func cliSet(ctx *cli.Context, evt hal.Evt, opts hal.Pref) {
	pref := opts.Set()
	// TODO: humanitarian formatting
	evt.Reply(fmt.Sprintf("%v", pref))
}

func httpPrefs(w http.ResponseWriter, r *http.Request) {
}
