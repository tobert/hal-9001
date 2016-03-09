package pagerduty

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
)

// https://developer.pagerduty.com/documentation/integration/events/trigger

var Endpoint = "https://events.pagerduty.com/generic/2010-04-15/create_event.json"

// Context is an interface for the contexts field in PD events.
type Context interface {
	GetType() string
}

type ContextLink struct {
	Type string `json:"type"`
	Href string `json:"href"`
	Text string `json:"text,omitempty"`
}

type ContextImage struct {
	Type string `json:"type"`
	Src  string `json:"src"`
	Href string `json:"href,omitempty"`
	Alt  string `json:"alt,omitempty"`
}

type Event struct {
	ServiceKey  string                 `json:"service_key"`
	EventType   string                 `json:"event_type"`
	Description string                 `json:"description"`
	IncidentKey string                 `json:"incident_key,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"` // arbitrary json
	Client      string                 `json:"client,omitempty"`
	ClientUrl   string                 `json:"client_url,omitempty"`
	Contexts    []Context              `json:"contexts,omitempty"`
}

type Response struct {
	Status      string   `json:"status"`
	Message     string   `json:"message"`
	IncidentKey string   `json:"incident_key,omitempty"`
	Errors      []string `json:"errors,omitempty"`
	StatusCode  int      `json:""`
}

// NewEvent returns an initialized Event structure. You probably don't
// want to use this and instead use NewTrigger/NewAck/NewResolve.
func NewEvent(serviceKey, eventType, description string) *Event {
	return &Event{
		ServiceKey:  serviceKey,
		EventType:   eventType,
		Description: description,
		Details:     make(map[string]interface{}),
		Contexts:    make([]Context, 0),
	}
}

func NewTrigger(serviceKey, description string) *Event {
	return NewEvent(serviceKey, "trigger", description)
}

func NewAck(serviceKey, description string) *Event {
	return NewEvent(serviceKey, "acknowledge", description)
}

func NewResolve(serviceKey, description string) *Event {
	return NewEvent(serviceKey, "resolve", description)
}

func NewResponse(status, message, incidentKey string) *Response {
	out := Response{
		Status:      status,
		Message:     message,
		IncidentKey: incidentKey,
		Errors:      make([]string, 0),
	}

	return &out
}

// Send posts the event to Pagerduty using the provided token.
func (e *Event) Send(token string) (*Response, error) {
	err := e.checkRequired()
	if err != nil {
		return e.respond("error", err.Error()), err
	}

	js, err := json.Marshal(e)
	if err != nil {
		log.Printf("json.Marshal failed: %s\n", err)
		return e.respond("error", err.Error()), err
	}

	resp, err := authenticatedPost(token, js)
	if err != nil {
		return e.respond("error", err.Error()), err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 200 {
		out := Response{}
		err = json.Unmarshal(body, &out)
		if err != nil {
			log.Printf("json.Unmarshal failed: %s\n", err)
			return nil, err
		}
		out.StatusCode = resp.StatusCode
		return &out, nil
	} else {
		msg := fmt.Sprintf("Server returned %d: %q", resp, string(body))
		return e.respond("error", msg), errors.New(msg)
	}
}

func (e *Event) respond(status, message string) *Response {
	return NewResponse(status, message, e.IncidentKey)
}

func (e *Event) checkRequired() error {
	et := e.EventType

	if len(et) == 0 {
		return errors.New("EventType is a required field.")
	}

	if et != "trigger" && et != "acknowledge" && et != "resolve" {
		msg := fmt.Sprintf("EventType must be one of 'trigger', 'acknowledge', or 'resolve'. Got: %q", et)
		return errors.New(msg)
	}

	if len(e.ServiceKey) == 0 {
		return errors.New("ServiceKey is a required field.")
	}

	if len(e.Description) == 0 {
		return errors.New("Description is a required field.")
	}

	return nil
}

func (c *ContextLink) GetType() string {
	return "link"
}

func (c *ContextImage) GetType() string {
	return "image"
}
