package autoresponder

import (
	"log"
	"time"

	"github.com/netflix/hal-9001/hal"
)

const NAME = "autoresponder"

const DEFAULT_MESSAGE = `Business hours are Monday to Friday, 9AM-6PM PST. If you need immediate
attention off hours, please page us with the !page command.`

const DEFAULT_TZ = "America/Los_Angeles"

func Register() {
	p := hal.Plugin{
		Name: NAME,
		Func: autoresponder,
		// match the starting < on Slack's mention messages but don't bother with
		// the second half and keep the RE simple for now
		Regex: "<[!@](?i:all|here|core)\\W",
	}

	p.Register()
}

// autoresponder is a handler that lets folks know that pinging the room
// outside of business hours does not have an SLA
func autoresponder(evt hal.Evt) {
	tz, err := time.LoadLocation(DEFAULT_TZ)
	if err != nil {
		log.Fatalf("Could not load timezone info for '%s': %s\n", DEFAULT_TZ, err)
	}

	// get the current time, making sure it's in the right timezone
	now := time.Now().In(tz)
	hr := now.Hour()
	wday := now.Weekday() // Sunday = 0

	// TODO: use preferences here
	//if wday == 0 || wday == 6 || hr < 9 || hr > 18 {
	if wday != 0 && hr != 0 { // testing shenanigans
		evt.Reply(DEFAULT_MESSAGE)
	}
}
