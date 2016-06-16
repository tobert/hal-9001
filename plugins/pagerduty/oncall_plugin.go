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

const OncallUsage = `!oncall <alias>

Find out who is oncall. If only one argument is provided, it must match
a known alias for a Pagerduty service. Otherwise, it is expected to be
a subcommand.

!oncall core
`

const DefaultTopicInterval = "10m"

// TODO: add the service key to the output such that someone trying to contact a team
// can page them from within Slack without having to set up a page alias or go out
// to a web page. !page should be able to take a service key so the output can include something
// like: "To page <team> use the command: !page <servicekey> <message>"
func oncall(msg hal.Evt) {
	parts := msg.BodyAsArgv()

	if len(parts) == 1 {
		msg.Reply(OncallUsage)
		return
	} else if len(parts) != 2 {
		msg.Replyf("%s: invalid command.\n%s", parts[0], OncallUsage)
		return
	}

	// make sure the pagerduty token is setup in hal.Secrets
	token, err := getSecrets()
	if err != nil || token == "" {
		msg.Replyf("pagerduty: %s is not set up in hal.Secrets. Cannot continue.", PagerdutyTokenKey)
		return
	}

	if parts[1] == "cache-now" {
		msg.Reply("Updating Pagerduty policy cache now.")
		cacheNow(token, msg.RoomId)
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
			svc, err := GetServiceByKey(token, aliasPref.Value)
			if err == nil {
			}
			// all through to search ...
		}
	*/

	// search over all policies looking for matching policy name, escalation
	// rule name, or service name
	matches := make([]Oncall, 0)
	oncalls := getOncallCache(token, false)
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

func getOncallCache(token string, forceUpdate bool) []Oncall {
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
	oncalls, err = GetOncalls(token, nil)
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

	token, err := getSecrets()
	if err != nil || token == "" {
		return // getSecrets will log the error
	}

	go func() {
		pf := hal.PeriodicFunc{
			Name:     cacheFuncName(i.RoomId),
			Interval: cd,
			Function: func() { cacheNow(token, i.RoomId) },
		}

		pf.Register()
		pf.Start()
	}()

	go func() {
		pf := hal.PeriodicFunc{
			Name:     topicFuncName(i.RoomId),
			Interval: td,
			Function: func() { topicUpdater(token, i.RoomId, i.Broker.Name()) },
		}

		pf.Register()
		pf.Start()
	}()

	// TODO: add a command to stop, etc.
}

func cacheNow(token, roomId string) {
	getOncallCache(token, true)
}

// topicUpdater runs periodically to update the topic in the room
// it's configured in.
// To fully enable it, you need the oncall schedule id from the pagerduty API.
// !prefs set --room * --broker slack --plugin pagerduty --key topic-updater-schedule-id --value <schedule id>
// !prefs set --room * --broker slack --plugin pagerduty --key topic-prefix --value <text>
// !prefs set --room * --broker slack --plugin pagerduty --key topic-suffix --value <text>
// TODO: see if there's a way to also resolve integration keys instead of using the schedule id
func topicUpdater(token, roomId, brokerName string) {
	log.Printf("ENTER topicUpdater(token, %q, %q)", roomId, brokerName)

	pref := hal.GetPref("", brokerName, roomId, "pagerduty", "topic-updater-schedule-id", "-")
	// probably not configured, nothing to see here...
	if !pref.Success || pref.Value == "-" {
		log.Printf("The pref ''/%q/%q/pagerduty/topic-updater-schedule-id does not seem to be set. Returning without taking action.",
			brokerName, roomId)
		return
	}

	params := map[string]string{
		"include[]":      "users",
		"schedule_ids[]": pref.Value,
	}

	oncalls, err := GetOncalls(token, params)
	if err != nil {
		log.Printf("Failed to fetch oncalls for schedule id %q: %s", pref.Value, err)
		return
	}

	log.Printf("Got %d users for schedule id %q", len(oncalls), pref.Value)

	// there may be more than one entry but if they're both on the same
	// schedule it should be the same primary oncall so ignore all but the first
	if len(oncalls) == 0 {
		log.Printf("no oncall results for id %q", pref.Value)
		return
	}

	// TODO: yet another place some kind of templating support would be handy
	prefix := hal.GetPref("", brokerName, roomId, "pagerduty", "topic-prefix", "")
	suffix := hal.GetPref("", brokerName, roomId, "pagerduty", "topic-suffix", "")

	// e.g. prefix = "", summary = "Al Tobey", suffix = " [team-dl@company.com] !pageus"
	topic := prefix.Value + oncalls[0].User.Summary + suffix.Value

	broker := hal.Router().GetBroker(brokerName)

	oldTopic, err := broker.GetTopic(roomId)
	if err != nil {
		log.Printf("Could not fetch current topic for room %q: %s", roomId, err)
		return
	}

	// only do the update if the topic has changed
	if topic != oldTopic {
		broker.SetTopic(roomId, topic)
	}
}

func cacheFuncName(roomId string) string {
	return fmt.Sprintf("pagerduty-cache-updater-%s", roomId)
}

func topicFuncName(roomId string) string {
	return fmt.Sprintf("pagerduty-topic-updater-%s", roomId)
}

// OncallsByLevel provides sorting by oncall level for []Oncall.
type OncallsByLevel []Oncall

func (a OncallsByLevel) Len() int      { return len(a) }
func (a OncallsByLevel) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a OncallsByLevel) Less(i, j int) bool {
	// sort "always on call" users to the end of the list
	if a[j].Schedule.Summary == "" {
		return true
	}

	return a[i].EscalationLevel < a[j].EscalationLevel
}

func formatOncallReply(wanted string, exactMatchFound bool, oncalls []Oncall) string {
	buf := bytes.NewBuffer([]byte{})

	if exactMatchFound {
		fmt.Fprintf(buf, "exact match found for %q\n", oncalls[0].EscalationPolicy.Summary)
	} else {
		fmt.Fprintf(buf, "%d records matched for query: %q\n", len(oncalls), wanted)
	}

	sort.Sort(OncallsByLevel(oncalls))

	for _, oncall := range oncalls {
		indent := strings.Repeat("    ", oncall.EscalationLevel)
		sched := oncall.Schedule.Summary
		if sched == "" {
			sched = "always on call"
		}

		if exactMatchFound {
			fmt.Fprintf(buf, "%s%s - %s\n", indent, oncall.User.Summary, sched)
		} else {
			fmt.Fprintf(buf, "%s%s - %s - %s\n", indent,
				oncall.EscalationPolicy.Summary, oncall.User.Summary, sched)
		}
	}

	return buf.String()
}
