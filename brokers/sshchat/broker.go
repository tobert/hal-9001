package sshchat

/*
 * Copyright 2019 Amy Tobey <tobert@gmail.com>
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import (
	"fmt"

	"github.com/tobert/hal-9001/hal"
)

var log hal.Logger

// Broker interacts with the sshchat service.
type Broker struct {
	inst string // the instance name of the broker
}

type Config struct {
	SSHUsername string // the ssh username
	SSHKeyFile  string // path to the private ssh key
}

func init() {
	log.SetPrefix("brokers/sshchat")
}

func (c Config) NewBroker(name string) Broker {
	b := Broker{
		inst: name,
	}
	return b
}

// Name returns the name of the broker as set in NewBroker.
func (b Broker) Name() string {
	return b.inst
}

func (b Broker) Send(evt hal.Evt) {
	log.Panic("Send() not implemented yet")
}

func (b Broker) SendAsSnippet(evt hal.Evt) {
	log.Panic("SendAsSnippet() not implemented yet")
}

// SendAsIs directly sends a message without considering it for posting as a snippet.
func (b Broker) SendAsIs(evt hal.Evt) {
	log.Panic("SendAsIs() not implemented yet")
}

func (b Broker) SendDM(evt hal.Evt) {
	log.Panic("SendDM() not implemented yet")
}

func (b Broker) Leave(roomId string) error {
	log.Panic("Leave() not implemented yet")
	return fmt.Errorf("nope")
}

func (b Broker) GetTopic(roomId string) (string, error) {
	log.Panic("GetTopic() not implemented yet")
	return "", fmt.Errorf("nope")
}

func (b Broker) SetTopic(roomId, topic string) error {
	log.Panic("SetTopic() not implemented yet")
	return fmt.Errorf("nope")
}

func (b Broker) SendTable(evt hal.Evt, hdr []string, rows [][]string) {
	out := evt.Clone()
	out.Body = hal.Utf8Table(hdr, rows)

	// in other brokers this might allow sending an image but that
	// doesn't matter here since we can count on monospace rendering
	b.SendAsIs(out)
}

// TODO: it might be fun to do ANSI formatting and support the image.fg and bg
// preferences like the Slack broker does, but for color terminals.
func (b Broker) SendAsImage(evt hal.Evt) {
	// just forward, same reason as SendTable
	b.SendAsIs(evt)
}

// usernames and ids are the same in sshchat
func (b Broker) LooksLikeRoomId(room string) bool {
	return true
}

func (b Broker) LooksLikeUserId(user string) bool {
	return true
}

// checks the cache to see if the room is known to this broker
func (b Broker) HasRoom(room string) bool {
	log.Panic("HasRoom() not implemented yet")
	return false
}

// Stream is an event loop for messages from the ssh channel.
func (b Broker) Stream(out chan *hal.Evt) {
}

func (b Broker) UserIdToName(id string) string {
	if id == "" {
		log.Debugf("UserIdToName(): Cannot look up empty string!")
		return ""
	}

	return id
}

func (b Broker) RoomIdToName(id string) string {
	if id == "" {
		log.Debugf("RoomIdToName(): Cannot look up empty string!")
		return ""
	}

	return id
}

func (b Broker) UserNameToId(name string) string {
	if name == "" {
		log.Debugf("UserNameToId(): Cannot look up empty string!")
		return ""
	}

	return name
}

func (b Broker) RoomNameToId(name string) string {
	if name == "" {
		log.Println("RoomNameToId(): Cannot look up empty string!")
		return ""
	}

	return name
}
