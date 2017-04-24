package pagerduty

/*
 * Copyright 2017 Netflix, Inc.
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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"time"
)

// https://v2.developer.pagerduty.com/docs/events-api-v2
const V2EventEndpoint = `https://events.pagerduty.com/v2/enqueue`

// data structures for the PagerDuty Common Event Format
type EventPayload struct {
	Summary   string            `json:"summary"`             // high-level text
	Severity  string            `json:"severity"`            // enum: info, warning, error, critical
	Source    string            `json:"source,omitempty"`    // e.g. hostname, IP, ARN
	Timestamp string            `json:"timestamp,omitempty"` // ISO8601
	Component string            `json:"component,omitempty"` // e.g. "mysql", "keepalive"
	Group     string            `json:"group,omitempty"`     // e.g. "www", "prod-data"
	Class     string            `json:"class,omitempty"`     // e.g. "High CPU", "Latency"
	Custom    map[string]string `json:"custom_details"`
}

type EventImage struct {
	Src  string `json:"src"`
	Href string `json:"href"`
	Alt  string `json:"alt"`
}

type EventBody struct {
	RoutingKey string       `json:"routing_key"`
	Action     string       `json:"event_action"`        // e.g. "trigger"
	DedupKey   string       `json:"dedup_key,omitempty"` // arbitrary key for server-side dedup
	Payload    EventPayload `json:"payload"`
	Images     []EventImage `json:"images"`
	Client     string       `json:"client"`     // e.g. "Scorebot/#core"
	ClientUrl  string       `json:"client_url"` // e.g. "https://scorebot.prod.netflix.net"
}

type EventResult struct {
	Status     string `json:"status"`    // e.g. "success"
	Message    string `json:"message"`   // e.g. "Event processed"
	DedupKey   string `json:"dedup_key"` // a uuid-ish key
	StatusCode int    `json:"-"`
}

func NewV2Event(routingKey string) *EventBody {
	now := time.Now()
	details := make(map[string]string)
	out := EventBody{
		RoutingKey: routingKey,
		Action:     "trigger",
		Payload: EventPayload{
			// provide defaults for required fields
			Summary:   "Something happened! This is the default summary.",
			Source:    "unspecified",
			Severity:  "error",
			Timestamp: now.Format(time.RFC3339),
			Custom:    details,
		},
		Images: []EventImage{},
	}

	return &out
}

func (eb *EventBody) Send(token string) (EventResult, error) {
	out := EventResult{Status: "failed"}

	err := eb.checkFields()
	if err != nil {
		return out, err
	}

	js, err := json.Marshal(eb)
	if err != nil {
		msg := fmt.Sprintf("json.Marshal failed: %s", err)
		out.Message = msg
		log.Println(msg)
		return out, err
	}

	resp, err := authenticatedPost(token, V2EventEndpoint, js)
	if err != nil {
		msg := fmt.Sprintf("POST failed: %s", err)
		out.Message = msg
		log.Println(msg)
		return out, err
	}
	defer resp.Body.Close()

	out.StatusCode = resp.StatusCode

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return out, err
	}

	if resp.StatusCode == 200 {
		err = json.Unmarshal(body, &out)
		if err != nil {
			msg := fmt.Sprintf("json.Unmarshal failed: %s", err)
			out.Status = "failed"
			out.Message = msg
			log.Println(msg)
			return out, err
		}
		return out, nil
	} else if resp.StatusCode == 202 {
		// 202 Accepted means they received the event but it's unclear whether it
		// will trigger an incident
		// Hopefully this will be a positive signal that a V1 key has been used...
		err = json.Unmarshal(body, &out)
		if err != nil {
			msg := fmt.Sprintf("json.Unmarshal failed: %s", err)
			out.Status = "failed"
			out.Message = msg
			log.Println(msg)
			return out, err
		}
		// return an error for 202 - it means Pagerduty isn't sure the alert
		// is going to trigger so callers need a strong signal on that
		return out, fmt.Errorf("Inconclusive response (202) from service.")
	} else {
		msg := fmt.Sprintf("Server returned %d: %q", resp, string(body))
		out.Message = msg
		return out, errors.New(msg)
	}
}

func (eb *EventBody) checkFields() error {
	// TODO: check some fields
	return nil
}
