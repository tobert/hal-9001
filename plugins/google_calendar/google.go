package google_calendar

import (
	"fmt"
	"time"

	"github.com/netflix/hal-9001/hal"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
)

const oauthJsonKey = `google-calendar-oauth-client-json`

// a simplified calendar event returned by getEvents
type CalEvent struct {
	Start       time.Time
	End         time.Time
	Name        string
	Description string
}

type GoogleError struct {
	Parent error
}

func (e GoogleError) Error() string {
	return fmt.Sprintf("Failed while communicating with Google Calender: %s", e.Parent.Error())
}

type PrefMissingError struct{}

func (e PrefMissingError) Error() string {
	return `the calendar-id pref must be set for the room. Try:
!prefs set --room * --plugin google_calendar --key calendar-id --value <id>`
}

type SecretMissingError struct{}

func (e SecretMissingError) Error() string {
	return "the google-calendar-oauth-client-json secret must be set. Contact the bot admin."
}

func getEvents(calendarId string, now time.Time) ([]CalEvent, error) {
	// TODO: figure out if it's feasible to have one secret per bot or
	// if it really needs to be per-calendar or room
	// TODO: this should probably be passed to this function rather than
	// making this file require hal
	secrets := hal.Secrets()
	jsonData := secrets.Get("google-calendar-oauth-client-json")
	if jsonData == "" {
		return nil, SecretMissingError{}
	}

	config, err := google.JWTConfigFromJSON([]byte(jsonData), calendar.CalendarReadonlyScope)
	if err != nil {
		return nil, GoogleError{err}
	}
	client := config.Client(oauth2.NoContext)
	cal, err := calendar.New(client)
	if err != nil {
		return nil, GoogleError{err}
	}

	min := now.Add(time.Hour * -1).Format(time.RFC3339)
	max := now.Add(time.Hour * 24).Format(time.RFC3339)
	events, err := cal.Events.List(calendarId).
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(min).
		TimeMax(max).
		Do()

	if err != nil {
		return nil, GoogleError{err}
	}

	out := make([]CalEvent, len(events.Items))
	for i, event := range events.Items {
		start, _ := time.Parse(time.RFC3339, event.Start.DateTime)
		out[i].Start = start
		end, _ := time.Parse(time.RFC3339, event.End.DateTime)
		out[i].End = end
		out[i].Name = event.Summary
		out[i].Description = event.Description
	}

	return out, nil
}
