package hal

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Evt is a generic container for events processed by the bot.
// Event sources are responsible for copying the appropriate data into
// the Evt fields. Routing and most plugins will not work if the body
// isn't copied, at a minimum.
// The original event should usually be attached to the Original
type Evt struct {
	Body     string      `json:"body"`    // body of the event, regardless of source
	Room     string      `json:"room"`    // the room where the event originated
	RoomId   string      `json:"room_id"` // the room id from the source broker
	User     string      `json:"user"`    // the username that created the event
	UserId   string      `json:"user_id"` // the user id from the source broker
	Time     time.Time   `json:"time"`    // timestamp of the event
	Brokers  Brokers     `json:"brokers"` // the stack of brokers the event has passed through
	Original interface{} // the original message container (e.g. slack.MessageEvent)
	instance *Instance   // used by the broker to provide plugin instance metadata
}

// Clone() returns a copy of the event with the same broker/room/user
// and a current timestamp. Body and Original will be empty.
func (e *Evt) Clone() Evt {
	out := Evt{
		Room:    e.Room,
		RoomId:  e.RoomId,
		User:    e.User,
		UserId:  e.UserId,
		Time:    time.Now(),
		Brokers: e.Brokers.Clone(), // TODO: consider reverting this back to just a single Broker:
	}

	return out
}

// Reply is a helper that crafts a new event from the provided string
// and initiates the reply on the broker attached to the event.
func (e *Evt) Reply(msg string) {
	out := e.Clone()
	out.Body = msg
	e.Brokers.Last().Send(out)
}

// Replyf is the same as Reply but allows for string formatting using
// fmt.Sprintf()
func (e *Evt) Replyf(msg string, a ...interface{}) {
	e.Reply(fmt.Sprintf(msg, a...))
}

// BrokerName returns the text name of current broker.
func (e *Evt) BrokerName() string {
	return e.Brokers.Last().Name()
}

// fetch union of all matching settings from the database
// for user, broker, room, and plugin
// Plugins can use the Prefs methods to filter from there.
func (e *Evt) FindPrefs() Prefs {
	broker := e.Brokers.Last().Name()
	plugin := e.instance.Plugin.Name
	return FindPrefs(e.User, broker, e.Room, plugin, "")
}

// gets the plugin instance's preferences
func (e *Evt) InstanceSettings() []Pref {
	broker := e.Brokers.Last().Name()
	plugin := e.instance.Plugin.Name

	out := make([]Pref, 0)

	for _, stg := range e.instance.Plugin.Settings {
		// ignore room-specific settings for other room
		if stg.Room != "" && stg.Room != e.Room {
			continue
		}

		pref := GetPref("", broker, e.Room, plugin, stg.Key, stg.Default)
		out = append(out, pref)
	}

	return out
}

// NewPref creates a new pref struct with user, room, broker, and plugin
// set using metadata from the event.
func (e *Evt) NewPref() Pref {
	return Pref{
		User:   e.User,
		Room:   e.Room,
		Broker: e.Brokers.Last().Name(),
		Plugin: e.instance.Plugin.Name,
	}
}

// BodyAsArgv does minimal parsing of the event body, returning an argv-like
// array of strings with quoted strings intact (but with quotes removed).
// The goal is shell-like, and is not a full implementation.
// Leading/trailing whitespace is removed.
// Escaping quotes, etc. is not supported.
func (e *Evt) BodyAsArgv() []string {
	// use a simple RE rather than pulling in a package to do this
	re := regexp.MustCompile(`'[^']*'|"[^"]*"|\S+`)
	body := strings.TrimSpace(e.Body)
	argv := re.FindAllString(body, -1)

	// remove the outer quotes from quoted strings
	for i, val := range argv {
		if strings.HasPrefix(val, `'`) && strings.HasSuffix(val, `'`) {
			tmp := strings.TrimPrefix(val, `'`)
			argv[i] = strings.TrimSuffix(tmp, `'`)
		} else if strings.HasPrefix(val, `"`) && strings.HasSuffix(val, `"`) {
			tmp := strings.TrimPrefix(val, `"`)
			argv[i] = strings.TrimSuffix(tmp, `"`)
		}
	}

	return argv
}

func (e *Evt) String() string {
	return fmt.Sprintf("%s/%s@%s: %s", e.User, e.Room, e.Time.String(), e.Body)
}
