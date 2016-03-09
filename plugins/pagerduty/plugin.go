package pagerduty

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"time"

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
const PAGERDUTY_TOKEN_KEY = `pagerduty.token`

// the hal.secrets key that should contain the pagerduty account domain
const PAGERDUTY_DOMAIN_KEY = `pagerduty.domain`

const PAGE_USAGE = `!page <alias> [optional message]

Send an alert via Pagerduty with an optional custom message.

!page core
!page core <message>
!pagecore HELP ME YOU ARE MY ONLY HOPE

!page add <alias> <service key>
!page rm <alias>
!page list
`

const ONCALL_USAGE = `!oncall <alias>

Find out who is oncall. If only one argument is provided, it must match
a known alias for a Pagerduty service. Otherwise, it is expected to be
a subcommand.

!oncall core
`

const PAGE_DEFAULT_MESSAGE = `HAL: your presence is requested in the chat room.`

const PAGE_ALIAS_KEY = `alias.%s`

func page(msg hal.Evt) {
	parts := msg.BodyAsArgv()

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

	switch parts[1] {
	case "h", "help":
		msg.Reply(PAGE_USAGE)
	case "add":
		addAlias(msg, parts[2:])
	case "rm":
		rmAlias(msg, parts[2:])
	case "list":
		listAlias(msg)
	default:
		pageAlias(msg, parts[1:])
	}
}

func pageAlias(msg hal.Evt, parts []string) {
	pageMessage := PAGE_DEFAULT_MESSAGE
	if len(parts) > 1 {
		pageMessage = strings.Join(parts, " ")
	}

	// map alias name to PD token via prefs
	qpref := msg.NewPref()
	qpref.User = ""
	qpref.Key = aliasKey(parts[0])
	pref := qpref.Get()

	// make sure the query succeeded
	if !pref.Success {
		log.Println("%s", pref.String())
		msg.Replyf("Unable to access preferences: %q", pref.Error)
		return
	}

	// if qpref.Get returned the default, the alias was not found
	if pref.Value == "" {
		msg.Replyf("Alias %q not recognized. Try !page add <alias> <service key>", parts[0])
		return
	}

	// get the Pagerduty auth token from the secrets API
	secrets := hal.Secrets()
	token := secrets.Get(PAGERDUTY_TOKEN_KEY)
	if token == "" {
		msg.Replyf("Your Pagerduty auth token does not seem to be configured. Please add the %q secret.",
			PAGERDUTY_TOKEN_KEY)
		return
	}

	// create the event and send it
	pde := NewTrigger(pref.Value, pageMessage) // in ./pagerduty.go
	resp, err := pde.Send(token)
	if err != nil {
		msg.Replyf("Error while communicating with Pagerduty. %d %s", resp.StatusCode, resp.Message)
		return
	}

	// TODO: add some boilerplate around this
	msg.Reply(resp.Message)
}

func addAlias(msg hal.Evt, parts []string) {
	if len(parts) != 2 {
		msg.Replyf("!page add requires 2 arguments, e.g. !page add sysadmins XXXXXXX")
		return
	}

	pref := msg.NewPref()
	pref.User = "" // filled in by NewPref and unwanted
	pref.Key = aliasKey(parts[0])
	pref.Value = parts[1]
	err := pref.Set()
	if err != nil {
		msg.Replyf("Write failed: %s", err)
	} else {
		msg.Replyf("Added alias: %q -> %q", parts[0], parts[1])
	}
}

func rmAlias(msg hal.Evt, parts []string) {
	if len(parts) != 1 {
		msg.Replyf("!page rm requires 1 argument, e.g. !page rm sysadmins")
		return
	}

	pref := msg.NewPref()
	pref.User = "" // filled in by NewPref and unwanted
	pref.Key = aliasKey(parts[0])
	pref.Delete()
}

func listAlias(msg hal.Evt) {
	pref := msg.NewPref()
	pref.User = "" // filled in by NewPref and unwanted
	prefs := pref.GetPrefs()
	data := prefs.Table()
	text := hal.AsciiTable(data[0], data[1:])
	msg.Reply(text)
}

func aliasKey(alias string) string {
	return fmt.Sprintf("alias.%s", alias)
}

func oncall(msg hal.Evt) {
	parts := msg.BodyAsArgv()

	if len(parts) == 1 {
		msg.Reply(ONCALL_USAGE)
		return
	} else if len(parts) != 2 {
		msg.Replyf("%s: invalid command.\n%s", parts[0], ONCALL_USAGE)
		return
	}

	secrets := hal.Secrets()
	token := secrets.Get(PAGERDUTY_TOKEN_KEY)
	if token == "" {
		msg.Replyf("Your Pagerduty auth token does not seem to be configured. Please add the %q secret.",
			PAGERDUTY_TOKEN_KEY)
		return
	}

	domain := secrets.Get(PAGERDUTY_DOMAIN_KEY)
	if domain == "" {
		msg.Replyf("Your Pagerduty domain does not seem to be configured. Please add the %q secret.",
			PAGERDUTY_DOMAIN_KEY)
		return
	}

	// let users know that they're going to have to wait for us to query the API
	if !IsPolicyCached() {
		msg.Reply("No cache available. Please wait, downloading policies from PagerDuty...")
	}

	// get all of the defined policies
	policies, err := GetEscalationPolicies(token, domain)
	if err != nil {
		msg.Replyf("REST call to Pagerduty failed: %s", err)
		return
	}

	want := strings.ToLower(parts[1])
	matches := make([]EscalationPolicy, 0)

	// search over all policies looking for matching policy name, escalation
	// rule name, or service name
	for _, policy := range policies {
		// try matching the policy name
		lname := strings.ToLower(policy.Name)
		if strings.Contains(lname, want) {
			matches = append(matches, policy)
			continue
		}

		// try matching the escalation rule names
		for _, rule := range policy.EscalationRules {
			lname = strings.ToLower(rule.RuleObject.Name)
			if strings.Contains(lname, want) {
				matches = append(matches, policy)
				continue
			}
		}

		// try matching service names
		for _, svc := range policy.Services {
			lname = strings.ToLower(svc.Name)
			if strings.Contains(lname, want) {
				matches = append(matches, policy)
				continue
			}
		}
	}

	reply := formatOncallReply(want, matches)
	msg.Reply(reply)
}

func formatOncallReply(wanted string, policies []EscalationPolicy) string {
	buf := bytes.NewBufferString(fmt.Sprintf("Results for %q\n", wanted))

	for _, policy := range policies {
		buf.WriteString(policy.Name)
		buf.WriteString("\n")

		for _, oncall := range policy.OnCall {
			times := formatTimes(oncall.Start, oncall.End)
			indent := strings.Repeat("  ", oncall.Level) // indent deeper per level
			user := fmt.Sprintf("  %s%s: %s %s\n", indent, oncall.User.Name, oncall.User.Email, times)
			buf.WriteString(user)
		}

		buf.WriteString("\n")
	}

	return buf.String()
}

func formatTimes(st, et *time.Time) string {
	var start, end string
	if st != nil {
		start = st.Local().Format("2006-01-02T15:04MST")
	} else {
		return "always on call"
	}

	if et != nil {
		end = et.Local().Format("2006-01-02T15:04MST")
	} else {
		return "always on call"
	}

	return fmt.Sprintf("%s - %s", start, end)
}
