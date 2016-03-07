package pagerduty

import (
	"fmt"
	"log"
	"strings"

	"github.com/netflix/hal-9001/hal"
)

func Register(gb *hal.GenericBroker) {
	pg := hal.Plugin{
		Name:   "page",
		Func:   page,
		Broker: gb,
		Regex:  "^[[:space:]]*!page",
	}
	pg.Register()

	oc := hal.Plugin{
		Name:   "oncall",
		Func:   oncall,
		Broker: gb,
		Regex:  "^[[:space:]]*!oncall",
	}
	oc.Register()
}

// the hal.secrets key that should contain the pagerduty auth token
const PAGERDUTY_SECRET_KEY = `pagerduty.token`

const PAGE_USAGE = `!page <team>

Send an alert via Pagerduty with an optional custom message.

e.g.

!page core
!page core <message>
`

const PAGE_DEFAULT_MESSAGE = `HAL: your presence is requested in the chat room.`

func page(msg hal.Evt) {
	parts := strings.Split(strings.TrimSpace(msg.Body), " ")

	// detect concatenated command + team name & split them
	// e.g. "!pagecore" -> {"!page", "core"}
	if strings.HasPrefix(parts[0], "!page") && len(parts[0]) > 5 {
		team := strings.TrimPrefix(parts[0], "!page")
		parts = append([]string{"!page", team}, parts[1:]...)
	}

	// should be 2 parts now, "!page" and the target team
	if parts[0] != "!page" || len(parts) < 2 {
		msg.Reply(PAGE_USAGE)
		return
	}

	team := parts[1]

	pageMessage := PAGE_DEFAULT_MESSAGE
	if len(parts) > 2 {
		pageMessage = strings.Join(parts, " ")
	}

	// map team name to PD token via prefs
	// TODO: figure out if it makes sense to teach prefmgr to be able to prefix
	// the team name with service_key when adding service keys, but ignore it
	// for now and manually enter them to get things going
	aliasKey := fmt.Sprintf("service_key.%s", team)
	pref := hal.GetPref("", "", "", "pagerduty", aliasKey, "NOPE")

	if !pref.Success {
		msg.Replyf("Unable to access preferences: %s", pref.Error)
		return
	}

	// if GetPref returned the default, the alias was not found
	if pref.Value == "NOPE" {
		msg.Replyf("Team %q not recognized.", team)
		return
	}

	// get the Pagerduty auth token from the secrets API
	secrets := hal.Secrets()
	token := secrets.Get(PAGERDUTY_SECRET_KEY)

	// create the event and send it
	pde := NewTrigger(pref.Value, pageMessage)
	resp, err := pde.Send(token)
	if err != nil {
		msg.Replyf("Error while communicating with Pagerduty. %d %s", resp.StatusCode, resp.Message)
	}

	// TODO: add some boilerplate around this
	msg.Reply(resp.Message)
}

func oncall(msg hal.Evt) {
	log.Printf("%s tried !oncall but it's not implemented yet.", msg.From)
	msg.Reply("Not implemented yet.")
}
