package slack

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"time"

	"github.com/netflix/hal-9001/hal"
	"github.com/nlopes/slack"
)

// Broker interacts with the slack service.
type Broker struct {
	Client  *slack.Client     // slack API object
	RTM     *slack.RTM        // slack RTM object
	inst    string            // broker instance name
	i2u     map[string]string // id->name cache
	i2c     map[string]string // id->name cache
	u2i     map[string]string // name->id cache
	c2i     map[string]string // name->id cache
	idRegex *regexp.Regexp    // compiled RE to match user/channel ids
}

type Config struct {
	Token string
}

func (c Config) NewBroker(name string) Broker {
	client := slack.New(c.Token)
	// TODO: check for failures and log.Fatalf()
	rtm := client.NewRTM()

	sb := Broker{
		Client:  client,
		RTM:     rtm,
		inst:    name,
		i2u:     make(map[string]string),
		i2c:     make(map[string]string),
		u2i:     make(map[string]string),
		c2i:     make(map[string]string),
		idRegex: regexp.MustCompile("^[UC][A-Z0-9]{8}$"),
	}

	// fill the caches at startup to cut down on API requests
	sb.FillUserCache()
	sb.FillChannelCache()

	go rtm.ManageConnection()

	return sb
}

// Name returns the name of the broker as set in NewBroker.
func (sb Broker) Name() string {
	return sb.inst
}

func (sb Broker) Send(evt hal.Evt) {
	// make sure the channel is an ID and not the name
	// TODO: go through plugins, etc and see if there's a sane way to make the ID persist through
	// the system and have the name only resolve in and out of the genericbroker and in plugins
	// but probably not ... this should be fine
	var channel string
	if sb.idRegex.MatchString(evt.Channel) {
		channel = evt.Channel
	} else {
		channel = sb.ChannelNameToId(evt.Channel)
	}

	om := sb.RTM.NewOutgoingMessage(evt.Body, channel)
	sb.RTM.SendMessage(om)
}

// Stream is an event loop for Slack events & messages from the RTM API.
// Events are copied to a hal.Evt and forwarded to the exchange where they
// can be processed by registered handlers.
func (sb Broker) Stream(out chan *hal.Evt) {
	for {
		select {
		case msg := <-sb.RTM.IncomingEvents:
			switch ev := msg.Data.(type) {
			case *slack.HelloEvent:
				log.Println("slack.HelloEvent") // ignored

			case *slack.ConnectedEvent:
				log.Printf("slack.ConnectedEvent (%s)\n", msg.Type) // ignored

			case *slack.MessageEvent:
				m := msg.Data.(*slack.MessageEvent)
				e := hal.Evt{
					Body:      m.Text,
					Channel:   sb.ChannelIdToName(m.Channel),
					ChannelId: m.Channel,
					From:      sb.UserIdToName(m.User),
					FromId:    m.User,
					Broker:    sb,
					Time:      slackTime(m.Timestamp),
					IsGeneric: true,
					Original:  m,
				}

				out <- &e

			case *slack.StarAddedEvent:
				sae := msg.Data.(*slack.StarAddedEvent)
				user := sb.UserIdToName(sae.User)

				e := hal.Evt{
					Body:      fmt.Sprintf("%q added a star", user),
					Channel:   sb.ChannelIdToName(sae.Item.Channel),
					ChannelId: sae.Item.Channel,
					From:      user,
					FromId:    sae.User,
					Broker:    sb,
					Time:      slackTime(sae.EventTimestamp),
					IsGeneric: false, // only available to slack-aware plugins
					Original:  sae,
				}

				out <- &e

			case *slack.StarRemovedEvent:
				sre := msg.Data.(*slack.StarRemovedEvent)
				user := sb.UserIdToName(sre.User)

				e := hal.Evt{
					Body:      fmt.Sprintf("%q removed a star", user),
					Channel:   sb.ChannelIdToName(sre.Item.Channel),
					ChannelId: sre.Item.Channel,
					From:      user,
					FromId:    sre.User,
					Broker:    sb,
					Time:      slackTime(sre.EventTimestamp),
					IsGeneric: false, // only available to slack-aware plugins
					Original:  sre,
				}

				out <- &e

			case *slack.ReactionAddedEvent:
				rae := msg.Data.(*slack.ReactionAddedEvent)
				user := sb.UserIdToName(rae.User)

				e := hal.Evt{
					Body:      fmt.Sprintf("%q added reaction %q", user, rae.Reaction),
					Channel:   sb.ChannelIdToName(rae.Item.Channel),
					ChannelId: rae.Item.Channel,
					From:      user,
					FromId:    rae.User,
					Broker:    sb,
					Time:      slackTime(rae.EventTimestamp),
					IsGeneric: false, // only available to slack-aware plugins
					Original:  rae,
				}

				out <- &e

			case *slack.ReactionRemovedEvent:
				rre := msg.Data.(*slack.ReactionAddedEvent)
				user := sb.UserIdToName(rre.User)

				e := hal.Evt{
					Body:      fmt.Sprintf("%q removed reaction %q", user, rre.Reaction),
					Channel:   sb.ChannelIdToName(rre.Item.Channel),
					ChannelId: rre.Item.Channel,
					From:      user,
					FromId:    rre.User,
					Broker:    sb,
					Time:      slackTime(rre.EventTimestamp),
					IsGeneric: false, // only available to slack-aware plugins
					Original:  rre,
				}

				out <- &e

			case *slack.PresenceChangeEvent:
				// ignored

			case *slack.LatencyReport:
				// ignored

			case *slack.RTMError:
				log.Printf("slack.RTMError: %s\n", ev.Error())

			case *slack.InvalidAuthEvent:
				log.Println("slack.InvalidAuthEvent")
				break

			default:
				log.Printf("Unexpected: %v\n", msg.Data)
			}
		}
	}
}

// slackTime converts the timestamp string to time.Time
// cribbed from: https://github.com/nlopes/slack/commit/17d746b30caa733b519f79fe372fd509bd6fc9fd
func slackTime(t string) time.Time {
	if t == "" {
		return time.Now()
	}

	floatN, err := strconv.ParseFloat(t, 64)
	if err != nil {
		log.Println("Error parsing Slack time string %q:", t, err)
		return time.Now()
	}

	return time.Unix(int64(floatN), 0)
}

func (sb *Broker) FillUserCache() {
	users, err := sb.Client.GetUsers()
	if err != nil {
		log.Printf("Failed to fetch user list: %s", err)
		return
	}

	for _, user := range users {
		sb.u2i[user.Name] = user.ID
		sb.i2u[user.ID] = user.Name
	}
}

func (sb *Broker) FillChannelCache() {
	channels, err := sb.Client.GetChannels(true)
	if err != nil {
		log.Printf("Failed to fetch channel list: %s", err)
		return
	}

	for _, channel := range channels {
		sb.c2i[channel.Name] = channel.ID
		sb.i2c[channel.ID] = channel.Name
	}
}

// UserIdToName gets the human-readable username for a user ID using an
// in-memory cache that falls through to the Slack API
func (sb Broker) UserIdToName(id string) string {
	if name, exists := sb.i2u[id]; exists {
		return name
	} else {
		user, err := sb.Client.GetUserInfo(id)
		if err != nil {
			log.Printf("Could not retrieve user info for '%s' from the slack API: %s\n", id, err)
			return ""
		}

		// TODO: verify if channel/user names are enforced unique in slack or if this is madness
		// remove this if it proves unnecessary (tobert/2016-03-02)
		if _, exists := sb.u2i[user.Name]; exists {
			if sb.u2i[user.Name] != user.ID {
				log.Fatalf("BUG: found a non-unique user name:ID pair. Had: %q/%q. Got: %q/%q",
					user.Name, sb.u2i[user.Name], user.Name, user.ID)
			}
		}

		sb.i2u[user.ID] = user.Name
		sb.i2u[user.Name] = user.ID

		return user.Name
	}
}

// ChannelIdToName gets the human-readable channel name for a user ID using an
// in-memory cache that falls through to the Slack API
func (sb Broker) ChannelIdToName(id string) string {
	if name, exists := sb.i2c[id]; exists {
		return name
	} else {
		channel, err := sb.Client.GetChannelInfo(id)
		if err != nil {
			log.Printf("Could not retrieve channel info for '%s' from the slack API: %s\n", id, err)
			return ""
		}

		// TODO: verify if channel/user names are enforced unique in slack or if this is madness
		// remove this if it proves unnecessary (tobert/2016-03-02)
		if _, exists := sb.c2i[channel.Name]; exists {
			if sb.c2i[channel.Name] != channel.ID {
				log.Fatalf("BUG: found a non-unique channel name:ID pair. Had: %q/%q. Got: %q/%q",
					channel.Name, sb.c2i[channel.Name], channel.Name, channel.ID)
			}
		}

		sb.i2c[channel.ID] = channel.Name
		sb.c2i[channel.Name] = channel.ID

		return channel.Name
	}
}

// UserNameToId gets the human-readable username for a user ID using an
// in-memory cache that falls through to the Slack API
func (sb Broker) UserNameToId(name string) string {
	if id, exists := sb.u2i[name]; exists {
		return id
	} else {
		// there doesn't seem to be a name->id lookup so refresh the cache
		// and try again if we get here
		sb.FillUserCache()
		if id, exists := sb.u2i[name]; exists {
			return id
		}

		log.Printf("Slack does not seem to have knowledge of username %q", name)
		return ""
	}
}

// ChannelNameToId gets the human-readable channel name for a user ID using an
// in-memory cache that falls through to the Slack API
func (sb Broker) ChannelNameToId(name string) string {
	if id, exists := sb.c2i[name]; exists {
		return id
	} else {
		sb.FillChannelCache()
		if id, exists := sb.c2i[name]; exists {
			return id
		}

		log.Printf("Slack does not seem to have knowledge of channel name %q", name)
		return ""
	}
}
