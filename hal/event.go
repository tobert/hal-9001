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
	Body      string      `json:"body"`       // body of the event, regardless of source
	Channel   string      `json:"channel"`    // the channel where the event originated
	ChannelId string      `json:"channel_id"` // the channel id from the source broker
	From      string      `json:"from"`       // the username that created the event
	FromId    string      `json:"from_id"`    // the user id from the source broker
	Time      time.Time   `json:"time"`       // timestamp of the event
	Broker    Broker      `json:"broker"`     // the broker origin of the event
	IsGeneric bool        `json:"is_generic"` // true if evt should be published to GenericBroker
	Original  interface{} // the original message container (e.g. slack.MessageEvent)
	instance  *Instance   // used by the broker to provide plugin instance metadata
}

// Clone() returns a copy of the event with the same broker/channel/from
// and a current timestamp. Body and Original will be empty.
func (e *Evt) Clone() Evt {
	out := Evt{
		Channel:   e.Channel,
		ChannelId: e.ChannelId,
		From:      e.From,
		FromId:    e.FromId,
		Time:      time.Now(),
		Broker:    e.Broker,
		IsGeneric: e.IsGeneric,
	}

	return out
}

// Reply is a helper that crafts a new event from the provided string
// and initiates the reply on the broker attached to the event.
func (e *Evt) Reply(msg string) {
	out := e.Clone()
	out.Body = msg
	e.Broker.Send(out)
}

// Replyf is the same as Reply but allows for string formatting using
// fmt.Sprintf()
func (e *Evt) Replyf(msg string, a ...interface{}) {
	e.Reply(fmt.Sprintf(msg, a...))
}

// BrokerName returns the text name of the broker.
func (e *Evt) BrokerName() string {
	return e.Broker.Name()
}

// fetch union of all matching settings from the database
// for user, broker, channel, and plugin
// Plugins can use the Prefs methods to filter from there.
func (e *Evt) FindPrefs() Prefs {
	broker := e.Broker.Name()
	plugin := e.instance.Plugin.Name
	return FindPrefs(e.From, broker, e.Channel, plugin, "")
}

// gets the plugin instance's preferences
func (e *Evt) InstanceSettings() []Pref {
	broker := e.Broker.Name()
	plugin := e.instance.Plugin.Name

	out := make([]Pref, 0)

	for _, stg := range e.instance.Plugin.Settings {
		// ignore channel-specific settings for other channels
		if stg.Channel != "" && stg.Channel != e.Channel {
			continue
		}

		pref := GetPref("", broker, e.Channel, plugin, stg.Key, stg.Default)
		out = append(out, pref)
	}

	return out
}

// NewPref creates a new pref struct with user, channel, broker, and plugin
// set using metadata from the event.
func (e *Evt) NewPref() Pref {
	return Pref{
		User:    e.From,
		Channel: e.Channel,
		Broker:  e.Broker.Name(),
		Plugin:  e.instance.Plugin.Name,
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
	return fmt.Sprintf("%s/%s@%s: %s", e.From, e.Channel, e.Time.String(), e.Body)
}
