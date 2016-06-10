package pagerduty

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
	"log"
	"sort"
	"strings"
	"time"

	"github.com/netflix/hal-9001/hal"
)

func Register() {
	pg := hal.Plugin{
		Name:  "page",
		Func:  page,
		Regex: "^[[:space:]]*!page",
	}
	pg.Register()

	oc := hal.Plugin{
		Name:  "oncall",
		Func:  oncall,
		Init:  oncallInit,
		Regex: "^[[:space:]]*!oncall",
	}
	oc.Register()
}

// the hal.secrets key that should contain the pagerduty auth token
const PagerdutyTokenKey = `pagerduty.token`

// the hal.secrets key that should contain the pagerduty account domain
const PagerdutyDomainKey = `pagerduty.domain`

// the key name used for caching the full escalation policy
const CacheKey = `pagerduty.policy_cache`

const PageUsage = `!page <alias> [optional message]

Send an alert via Pagerduty with an optional custom message.

Aliases that have a comma-separated list of service keys will result in one page going to each service key when the alias is paged.

!page core
!page core <message>
!pagecore HELP ME YOU ARE MY ONLY HOPE

!page add <alias> <service key>
!page add <alias> <service key>,<service_key>,<service_key>,...
!page rm <alias>
!page list
`

const OncallUsage = `!oncall <alias>

Find out who is oncall. If only one argument is provided, it must match
a known alias for a Pagerduty service. Otherwise, it is expected to be
a subcommand.

!oncall core
`

const PageDefaultMessage = `HAL: your presence is requested in the chat room.`

const cacheExpire = time.Minute * 10

const DefaultCacheInterval = "1h"
const DefaultTopicInterval = "1h"

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
		msg.Reply(PageUsage)
		return
	}

	switch parts[1] {
	case "h", "help":
		msg.Reply(PageUsage)
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
	pageMessage := PageDefaultMessage
	if len(parts) > 1 {
		pageMessage = strings.Join(parts, " ")
	}

	// map alias name to PD token via prefs
	key := aliasKey(parts[0])
	pref := msg.AsPref().FindKey(key).One()

	// make sure the query succeeded
	if !pref.Success {
		log.Printf("Unable to access preferences: %#q", pref.Error)
		msg.Replyf("Unable to access preferences: %#q", pref.Error)
		return
	}

	// if qpref.Get returned the default, the alias was not found
	if pref.Value == "" {
		msg.Replyf("Alias %q not recognized. Try !page add <alias> <service key>", parts[0])
		return
	}

	// make sure the hal secrets are set up
	token, _, err := getSecrets()
	if err != nil {
		msg.Error(err)
		return
	}

	// the value can be a list of tokens, separated by commas
	for _, svckey := range strings.Split(pref.Value, ",") {
		// create the event and send it
		pde := NewTrigger(svckey, pageMessage) // in ./pagerduty.go
		resp, err := pde.Send(token)
		if err != nil {
			msg.Replyf("Error while communicating with Pagerduty. %d %s", resp.StatusCode, resp.Message)
			return
		}

		log.Printf("Pagerduty response message: %s\n", resp.Message)
	}

	msg.Replyf("Notification sent.")
}

func addAlias(msg hal.Evt, parts []string) {
	if len(parts) < 2 {
		msg.Replyf("!page add requires 2 arguments, e.g. !page add sysadmins XXXXXXX")
		return
	} else if len(parts) > 2 {
		keys := strings.Replace(strings.Join(parts[1:], ","), ",,", ",", len(parts)-2)
		parts = []string{parts[0], keys}
	}

	pref := msg.AsPref()
	pref.User = "" // filled in by AsPref and unwanted
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

	pref := msg.AsPref()
	pref.User = "" // filled in by AsPref and unwanted
	pref.Key = aliasKey(parts[0])
	pref.Delete()

	msg.Replyf("Removed alias %q", parts[0])
}

func listAlias(msg hal.Evt) {
	pref := msg.AsPref()
	pref.User = "" // filled in by AsPref and unwanted
	prefs := pref.GetPrefs()
	data := prefs.Table()
	msg.ReplyTable(data[0], data[1:])
}

func aliasKey(alias string) string {
	return fmt.Sprintf("alias.%s", alias)
}

func oncall(msg hal.Evt) {
	parts := msg.BodyAsArgv()

	if len(parts) == 1 {
		msg.Reply(OncallUsage)
		return
	} else if len(parts) != 2 {
		msg.Replyf("%s: invalid command.\n%s", parts[0], OncallUsage)
		return
	}

	// make sure the pagerduty token and domain are setup in hal.Secrets
	token, domain, err := getSecrets()
	if err != nil || token == "" || domain == "" {
		msg.Replyf("pagerduty: Either the %s or %s is not set up in hal.Secrets. Cannot continue.",
			PagerdutyTokenKey, PagerdutyDomainKey)
		return
	}

	if parts[1] == "cache-now" {
		msg.Reply("Updating Pagerduty policy cache now.")
		cacheNow(token, domain, msg.RoomId)
		msg.Reply("Pagerduty policy cache update complete.")
		return
	} else if parts[1] == "cache-status" {
		age := int(hal.Cache().Age(CacheKey).Seconds())
		next := time.Time{}
		status := "broken"
		pf := hal.GetPeriodicFunc(cacheFuncName(msg.RoomId))
		if pf != nil {
			next = pf.Last().Add(pf.Interval)
			status = pf.Status()
		}
		msg.Replyf("The cache is %d seconds old. Auto-update is %s and its next update is at %s.",
			age, status, next.Format(time.UnixDate))
		return
	}

	// TODO: look at the aliases set up for !page and try for an exact match
	// before doing fuzzy search -- move fuzzy search to a "search" subcommand
	// so it's clear that it is not precise
	want := strings.ToLower(parts[1])

	// see if there's an exact match on an alias, e.g. "!oncall core" -> alias.core
	/*
		aliasPref := msg.AsPref().SetUser("").FindKey(aliasKey(want)).One()
		if aliasPref.Success {
			svc, err := GetServiceByKey(token, domain, aliasPref.Value)
			if err == nil {
			}
			// all through to search ...
		}
	*/

	// search over all policies looking for matching policy name, escalation
	// rule name, or service name
	matches := make([]Oncall, 0)
	oncalls := getOncallCache(token, domain, false)
	var exactMatchFound bool

	for _, oncall := range oncalls {
		schedSummary := strings.ToLower(oncall.Schedule.Summary)
		if schedSummary == want {
			matches = append(matches, oncall)
			exactMatchFound = true
			continue
		} else if !exactMatchFound && strings.Contains(schedSummary, want) {
			matches = append(matches, oncall)
			continue
		}

		epSummary := strings.ToLower(oncall.EscalationPolicy.Summary)
		if epSummary == want {
			matches = append(matches, oncall)
			exactMatchFound = true
			continue
		} else if !exactMatchFound && strings.Contains(epSummary, want) {
			matches = append(matches, oncall)
			continue
		}
	}

	reply := formatOncallReply(want, exactMatchFound, matches)
	msg.Reply(reply)
}

// TODO: consider making the token key per-room so different rooms can use different tokens
// doing this will require a separate cache object per token...
func getSecrets() (token, domain string, err error) {
	secrets := hal.Secrets()
	token = secrets.Get(PagerdutyTokenKey)
	if token == "" {
		err = fmt.Errorf("Your Pagerduty auth token does not seem to be configured. Please add the %q secret.", PagerdutyTokenKey)
	}

	domain = secrets.Get(PagerdutyDomainKey)
	if domain == "" {
		err = fmt.Errorf("Your Pagerduty domain does not seem to be configured. Please add the %q secret.", PagerdutyDomainKey)
	}

	if err != nil {
		log.Println(err)
	}

	return token, domain, err
}

func getOncallCache(token, domain string, forceUpdate bool) []Oncall {
	oncalls := []Oncall{}

	// see if there's a copy cached
	if hal.Cache().Exists(CacheKey) {
		ttl, err := hal.Cache().Get(CacheKey, &oncalls)
		if err != nil {
			log.Printf("Error retreiving oncalls from the Hal TTL cache: %s", err)
			oncalls = []Oncall{}
		} else if ttl == 0 || forceUpdate {
			oncalls = []Oncall{}
		}
	}

	// the cache exists and is still valid, return it now
	if len(oncalls) > 0 {
		return oncalls
	}

	// get all of the defined policies
	var err error
	oncalls, err = GetOncalls(token, domain)
	if err != nil {
		log.Printf("Returning empty list. REST call to Pagerduty failed: %s", err)
		return []Oncall{}
	}

	// always update the cache regardless of ttl
	hal.Cache().Set(CacheKey, &oncalls, cacheExpire)

	return oncalls
}

func oncallInit(i *hal.Instance) {
	cacheFreq := hal.GetPref("", "", i.RoomId, "pagerduty", "cache-update-frequency", DefaultCacheInterval)
	cd, err := time.ParseDuration(cacheFreq.Value)
	if err != nil {
		log.Panicf("BUG: could not parse cache update frequency preference: %q", cacheFreq.Value)
	}

	topicFreq := hal.GetPref("", "", i.RoomId, "pagerduty", "topic-update-frequency", DefaultTopicInterval)
	td, err := time.ParseDuration(topicFreq.Value)
	if err != nil {
		log.Panicf("BUG: could not parse topic update frequency preference: %q", topicFreq.Value)
	}

	token, domain, err := getSecrets()
	if err != nil || token == "" || domain == "" {
		return // getSecrets will log the error
	}

	go func() {
		pf := hal.PeriodicFunc{
			Name:     cacheFuncName(i.RoomId),
			Interval: cd,
			Function: func() { cacheNow(token, domain, i.RoomId) },
		}

		pf.Register()
		pf.Start()
	}()

	go func() {
		pf := hal.PeriodicFunc{
			Name:     topicFuncName(i.RoomId),
			Interval: td,
			Function: func() { topicUpdater(token, domain, i.RoomId) },
		}

		pf.Register()
		pf.Start()
	}()

	// TODO: add a command to stop, etc.
}

func cacheNow(token, domain, roomId string) {
	getOncallCache(token, domain, true)
}

func topicUpdater(token, domain, roomId string) {
}

func cacheFuncName(roomId string) string {
	return fmt.Sprintf("pagerduty-cache-updater-%s", roomId)
}

func topicFuncName(roomId string) string {
	return fmt.Sprintf("pagerduty-topic-updater-%s", roomId)
}

// OncallsByLevel provides sorting by oncall level for []Oncall.
type OncallsByLevel []Oncall

func (a OncallsByLevel) Len() int           { return len(a) }
func (a OncallsByLevel) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a OncallsByLevel) Less(i, j int) bool { return a[i].EscalationLevel < a[j].EscalationLevel }

func formatOncallReply(wanted string, exactMatchFound bool, oncalls []Oncall) string {
	age := int(hal.Cache().Age(CacheKey).Seconds())
	buf := bytes.NewBuffer([]byte{})

	if exactMatchFound {
		fmt.Fprintf(buf, "exact match found for %q\n", oncalls[0].EscalationPolicy.Summary)
	} else {
		fmt.Fprintf(buf, "%d escalation policies matched %q\n", len(oncalls), wanted)
	}

	sort.Sort(OncallsByLevel(oncalls))

	for _, oncall := range oncalls {
		indent := strings.Repeat("    ", oncall.EscalationLevel)
		sched := oncall.Schedule.Summary
		if sched == "" {
			sched = "always on call"
		}

		if exactMatchFound {
			fmt.Fprintf(buf, "%s%s - %s\n", indent,
				oncall.User.Summary, sched)
		} else {
			fmt.Fprintf(buf, "%s%s - %s - %s\n", indent,
				oncall.EscalationPolicy.Summary, oncall.User.Summary, sched)
		}
	}

	return buf.String()
}

func formatTimes(st, et *time.Time) string {
	var start, end string
	if st != nil {
		start = st.Local().Format("2006-01-02")
	} else {
		return "always on call"
	}

	if et != nil {
		end = et.Local().Format("2006-01-02")
	} else {
		return "always on call"
	}

	return fmt.Sprintf("%s - %s", start, end)
}
