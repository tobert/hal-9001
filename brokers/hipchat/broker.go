package hipchat

import (
	"log"
	"strings"
	"time"

	"github.com/mattn/go-xmpp"
	"github.com/netflix/hal-9001/hal"
)

// Broker contains the Hipchat API handles required for interacting
// with the hipchat service.
type Broker struct {
	Client *xmpp.Client
	inst   string
}

type Config struct {
	Host     string
	Jid      string
	Password string
	Channels map[string]string
}

// HIPCHAT_HOST is the only supported hipchat host.
const HIPCHAT_HOST = `chat.hipchat.com:5223`

// Hipchat is a singleton that returns an initialized and connected
// Broker. It can be called anywhere in the bot at any time.
// Host must be "chat.hipchat.com:5223". This requirement can go away
// once someone takes the time to integrate and test against an on-prem
// Hipchat server.
func (c Config) NewBroker(name string) Broker {
	// TODO: remove this once the TLS/SSL requirements are sorted
	if c.Host != HIPCHAT_HOST {
		log.Println("TODO: Only SSL and hosted Hipchat are supported at the moment.")
		log.Printf("Hipchat host must be %q.", HIPCHAT_HOST)
	}

	// for some reason Go's STARTTLS seems to be incompatible with
	// Hipchat's or maybe Hipchat TLS is broken, so don't bother and use SSL.
	options := xmpp.Options{
		Host:          c.Host,
		User:          c.Jid,
		Debug:         false,
		Password:      c.Password,
		Resource:      "bot",
		Session:       true,
		Status:        "Available",
		StatusMessage: "Hal-9001 online.",
	}

	client, err := options.NewClient()
	if err != nil {
		log.Fatalf("Could not connect to Hipchat over XMPP: %s\n", err)
	}

	for jid, name := range c.Channels {
		client.JoinMUC(jid, name)
	}

	hb := Broker{
		Client: client,
		inst:   name,
	}

	return hb
}

func (hb Broker) Name() string {
	return hb.inst
}

func (hb Broker) Send(evt hal.Evt) {
	msg := xmpp.Chat{
		Text:  evt.Body,
		Stamp: evt.Time,
	}

	_, err := hb.Client.Send(msg)
	if err != nil {
		log.Printf("Failed to send message to Hipchat server: %s\n", err)
	}
}

// Subscribe joins a channel with the given alias.
// These names are specific to how Hipchat does things.
func (hb *Broker) Subscribe(channel, alias string) {
	// TODO: take a channel name and somehow look up the goofy MUC name
	// e.g. client.JoinMUC("99999_channelName@conf.hipchat.com", "Bot Name")
	hb.Client.JoinMUC(channel, alias)
}

// Keepalive is a timer loop that can be fired up to periodically
// send keepalive messages to the Hipchat server in order to prevent
// Hipchat from shutting the connection down due to inactivity.
func (hb *Broker) heartbeat(t time.Time) {
	msg := xmpp.Chat{Text: "heartbeat"}
	msg.Stamp = t

	n, err := hb.Client.Send(msg)
	if err != nil {
		log.Fatalf("Failed to send keepalive (%d): %s\n", n, err)
	}
}

// Stream is an event loop for Hipchat events.
func (hb Broker) Stream(out chan *hal.Evt) {
	client := hb.Client
	incoming := make(chan *xmpp.Chat)
	timer := time.Tick(time.Minute * 1) // once a minute

	// grab chat messages using the blocking Recv() and forward them
	// on a channel so the select loop can also handle sending heartbeats
	go func() {
		for {
			msg, err := client.Recv()
			if err != nil {
				log.Printf("Error receiving from Hipchat: %s\n", err)
			}

			switch t := msg.(type) {
			case xmpp.Chat:
				m := msg.(xmpp.Chat)
				incoming <- &m
			case xmpp.Presence:
				continue // ignored
			default:
				log.Printf("Unhandled message of type '%T': %s ", t, t)
			}
		}
	}()

	for {
		select {
		case t := <-timer:
			hb.heartbeat(t)
		case chat := <-incoming:
			// Remote should look like "99999_channelName@conf.hipchat.com/User Name"
			parts := strings.SplitN(chat.Remote, "/", 2)

			if len(parts) == 2 {
				e := hal.Evt{
					Body:      chat.Text,
					Channel:   parts[0], // TODO: provide the human-readable name
					ChannelId: parts[0],
					From:      parts[1],
					FromId:    parts[1],   // TODO: provide the JID
					Time:      time.Now(), // m.Stamp seems to be zeroed
					IsGeneric: true,
					Original:  &chat,
				}

				out <- &e
			}
		}
	}
}

// required by interface
// TODO: replace these with actually useful versions
func (b Broker) ChannelIdToName(in string) string { return in }
func (b Broker) ChannelNameToId(in string) string { return in }
func (b Broker) UserIdToName(in string) string    { return in }
func (b Broker) UserNameToId(in string) string    { return in }
