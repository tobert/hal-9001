// cross_the_streams replicates messages between brokers
package cross_the_streams

import (
	"fmt"
	"log"

	"github.com/netflix/hal-9001/hal"
)

// Register makes this plugin available to the system.
func Register() {
	plugin := hal.Plugin{
		Name:  "cross_the_streams",
		Func:  crossStreams,
		Regex: "", // get all messages
		//  source: Pref.Room / Pref.Broker
		Settings: hal.Prefs{
			hal.Pref{Plugin: "cross_the_streams", Key: "to.broker"},
			hal.Pref{Plugin: "cross_the_streams", Key: "to.room"},
		},
	}

	plugin.Register()
}

// crossStreams looks at events it recieves and repeats them
// to a different broker.
func crossStreams(evt hal.Evt) {
	prefs := evt.InstanceSettings()
	tbPrefs := prefs.Key("to.broker")
	trPrefs := prefs.Key("to.room")

	// no matches, move on
	if len(tbPrefs) == 0 || len(trPrefs) == 0 {
		return
	}

	toBroker := tbPrefs[0].Value
	toRoomId := trPrefs[0].Value

	tb := hal.Router().GetBroker(toBroker)
	if tb != nil {
		toRoom := tb.RoomIdToName(toRoomId)
		body := fmt.Sprintf("%s %s@%s: %s", evt.Time, evt.User, evt.Room, evt.Body)
		out := hal.Evt{
			Body:   body,
			Room:   toRoom,
			RoomId: toRoomId,
			Time:   evt.Time,
			Broker: tb,
		}
		tb.Send(out)
	} else {
		log.Printf("hal.Router does not know about a broker named %q", toBroker)
	}
}
