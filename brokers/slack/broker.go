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
	idRegex *regexp.Regexp    // compiled RE to match user/room ids
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
	sb.FillRoomCache()

	go rtm.ManageConnection()

	return sb
}

// Name returns the name of the broker as set in NewBroker.
func (sb Broker) Name() string {
	return sb.inst
}

func (sb Broker) Send(evt hal.Evt) {
	om := sb.RTM.NewOutgoingMessage(evt.Body, evt.RoomId)
	sb.RTM.SendMessage(om)
}

// checks the cache to see if the room is known to this broker
func (sb Broker) HasRoom(room string) bool {
	if sb.idRegex.MatchString(room) {
		_, exists := sb.i2c[room]
		return exists
	} else {
		_, exists := sb.c2i[room]
		return exists
	}
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
				log.Println("brokers/slack ignoring HelloEvent")

			case *slack.ConnectedEvent:
				log.Printf("brokers/slack ignoring ConnectedEvent")

			case *slack.MessageEvent:
				m := msg.Data.(*slack.MessageEvent)
				// slack channels = hal rooms, see hal-9001/hal/event.go
				e := hal.Evt{
					Body:      m.Text,
					Room:      sb.RoomIdToName(m.Channel),
					RoomId:    m.Channel,
					User:      sb.UserIdToName(m.User),
					UserId:    m.User,
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
					Room:      sb.RoomIdToName(sae.Item.Channel),
					RoomId:    sae.Item.Channel,
					User:      user,
					UserId:    sae.User,
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
					Room:      sb.RoomIdToName(sre.Item.Channel),
					RoomId:    sre.Item.Channel,
					User:      user,
					UserId:    sre.User,
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
					Room:      sb.RoomIdToName(rae.Item.Channel),
					RoomId:    rae.Item.Channel,
					User:      user,
					UserId:    rae.User,
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
					Room:      sb.RoomIdToName(rre.Item.Channel),
					RoomId:    rre.Item.Channel,
					User:      user,
					UserId:    rre.User,
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
				log.Printf("brokers/slack ignoring RTMError: %s\n", ev.Error())

			case *slack.InvalidAuthEvent:
				log.Println("brokers/slack InvalidAuthEvent")
				break

			default:
				log.Printf("brokers/slack: unexpected message: %+v\n", msg)
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
		log.Println("brokers/slack error parsing Slack time string %q:", t, err)
		return time.Now()
	}

	return time.Unix(int64(floatN), 0)
}

func (sb *Broker) FillUserCache() {
	users, err := sb.Client.GetUsers()
	if err != nil {
		log.Printf("brokers/slack failed to fetch user list: %s", err)
		return
	}

	for _, user := range users {
		sb.u2i[user.Name] = user.ID
		sb.i2u[user.ID] = user.Name
	}
}

func (sb *Broker) FillRoomCache() {
	rooms, err := sb.Client.GetChannels(true)
	if err != nil {
		log.Printf("brokers/slack failed to fetch room list: %s", err)
		return
	}

	for _, room := range rooms {
		sb.c2i[room.Name] = room.ID
		sb.i2c[room.ID] = room.Name
	}
}

// UserIdToName gets the human-readable username for a user ID using an
// in-memory cache that falls through to the Slack API
func (sb Broker) UserIdToName(id string) string {
	if id == "" {
		log.Println("broker/slack/UserIdToName(): Cannot look up empty string!")
		return ""
	}

	if name, exists := sb.i2u[id]; exists {
		return name
	} else {
		user, err := sb.Client.GetUserInfo(id)
		if err != nil {
			log.Printf("brokers/slack could not retrieve user info for '%s' via API: %s\n", id, err)
			return ""
		}

		// TODO: verify if room/user names are enforced unique in slack or if this is madness
		// remove this if it proves unnecessary (tobert/2016-03-02)
		if _, exists := sb.u2i[user.Name]; exists {
			if sb.u2i[user.Name] != user.ID {
				log.Fatalf("BUG(brokers/slack): found a non-unique user name:ID pair. Had: %q/%q. Got: %q/%q",
					user.Name, sb.u2i[user.Name], user.Name, user.ID)
			}
		}

		sb.i2u[user.ID] = user.Name
		sb.i2u[user.Name] = user.ID

		return user.Name
	}
}

// RoomIdToName gets the human-readable room name for a user ID using an
// in-memory cache that falls through to the Slack API
func (sb Broker) RoomIdToName(id string) string {
	if id == "" {
		log.Println("broker/slack/RoomIdToName(): Cannot look up empty string!")
		return ""
	}

	if name, exists := sb.i2c[id]; exists {
		return name
	} else {
		room, err := sb.Client.GetChannelInfo(id)
		if err != nil {
			log.Printf("brokers/slack could not retrieve room info for '%s' via API: %s\n", id, err)
			return ""
		}

		// TODO: verify if room/user names are enforced unique in slack or if this is madness
		// remove this if it proves unnecessary (tobert/2016-03-02)
		if _, exists := sb.c2i[room.Name]; exists {
			if sb.c2i[room.Name] != room.ID {
				log.Fatalf("BUG(brokers/slack): found a non-unique room name:ID pair. Had: %q/%q. Got: %q/%q",
					room.Name, sb.c2i[room.Name], room.Name, room.ID)
			}
		}

		sb.i2c[room.ID] = room.Name
		sb.c2i[room.Name] = room.ID

		return room.Name
	}
}

// UserNameToId gets the human-readable username for a user ID using an
// in-memory cache that falls through to the Slack API
func (sb Broker) UserNameToId(name string) string {
	if name == "" {
		log.Println("broker/slack/UserNameToId(): Cannot look up empty string!")
		return ""
	}

	if id, exists := sb.u2i[name]; exists {
		return id
	} else {
		// there doesn't seem to be a name->id lookup so refresh the cache
		// and try again if we get here
		sb.FillUserCache()
		if id, exists := sb.u2i[name]; exists {
			return id
		}

		log.Printf("brokers/slack service does not seem to have knowledge of username %q", name)
		return ""
	}
}

// RoomNameToId gets the human-readable room name for a user ID using an
// in-memory cache that falls through to the Slack API
func (sb Broker) RoomNameToId(name string) string {
	if name == "" {
		log.Println("broker/slack/RoomNameToId(): Cannot look up empty string!")
		return ""
	}

	if id, exists := sb.c2i[name]; exists {
		return id
	} else {
		sb.FillRoomCache()
		if id, exists := sb.c2i[name]; exists {
			return id
		}

		log.Printf("brokers/slack service does not seem to have knowledge of room name %q", name)
		return ""
	}
}
