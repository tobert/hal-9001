// cross_the_streams replicates messages between brokers
package cross_the_streams

import (
	"fmt"

	"github.com/netflix/hal-9001/hal"
)

// Register makes this plugin available to the system.
func Register(gb hal.GenericBroker) {
	plugin := hal.Plugin{
		Name:   "cross_the_streams",
		Func:   crossStreams,
		Regex:  "", // get all messages
		Broker: gb,
		Settings: map[string]string{
			"from_broker": "", // the string name of the source broker
			"to_broker":   "", // the string name of the destination broker
			"to_channel":  "", // the channel on the destination broker
		},
	}

	plugin.Register()
}

// crossStreams looks at events it recieves and repeats them
// to a different broker.
func crossStreams(evt hal.Evt) {
	broker := evt.Broker.Name()
	settings := evt.InstanceSettings()
	router := hal.Router()

	// if the plugin is active without config, this should just be a harmless waste of time
	if to, exists := settings["to_broker"]; exists {
		if from, exists := settings["from_broker"]; exists {

			// might be better to require a to_channel but for now prefer convenience
			var channel string
			if channel, exists = settings["to_channel"]; exists {
				channel = settings["to_channel"]
			} else {
				channel = evt.Channel
			}

			if broker == from {
				tb := router.GetBroker(to)
				if tb != nil {
					out := hal.Evt{
						Body:    fmt.Sprintf("%s %s@%s: %s", evt.Time, evt.From, from, evt.Body),
						Channel: channel,
						From:    evt.From, // ignored (for now)
						Time:    evt.Time,
						Broker:  tb,
					}
					tb.Send(out)
				}
			}
		}
	}
}
