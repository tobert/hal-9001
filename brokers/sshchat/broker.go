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
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"regexp"
	"strings"
	"time"

	"github.com/tobert/hal-9001/hal"
	"golang.org/x/crypto/ssh"
)

const ansiCleanReSrc = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"
const parseMsgReSrc = `^\[\w+\] (\w+): (.*)\s*$`

var log hal.Logger
var ansiCleanRe, parseMsgRe *regexp.Regexp

// Broker interacts with the sshchat service.
type Broker struct {
	inst       string // the instance name of the broker
	sshConfig  *ssh.ClientConfig
	sshClient  *ssh.Client
	sshSession *ssh.Session
	stdin      chan string
	stdout     chan string
	stderr     chan string
}

type Config struct {
	SSHUsername string // the ssh username
	SSHKeyFile  string // path to the private ssh key
}

func init() {
	log.SetPrefix("brokers/sshchat")
	ansiCleanRe = regexp.MustCompile(ansiCleanReSrc)
	parseMsgRe = regexp.MustCompile(parseMsgReSrc)
}

func (c Config) NewBroker(name string) Broker {
	var sshConf ssh.ClientConfig

	sshConf.SetDefaults()
	sshConf.User = c.SSHUsername
	sshConf.Auth = c.loadSSHKey()
	sshConf.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	// TODO: don't hardcode the address
	client, err := ssh.Dial("tcp", "localhost:2022", &sshConf)
	if err != nil {
		log.Fatalf("Unable to connect to sshchat server: %s", err)
	} else {
		log.Printf("ohai")
	}

	sess, err := client.NewSession()
	if err != nil {
		log.Fatalf("Could not create an ssh session: %s", err)
	}
	// TODO: session.Close()

	// create buffered channels that will be fed by goroutines
	// that tranlate to the ssh session's stdio
	inChan := make(chan string, 10)
	outChan := make(chan string, 10)
	errChan := make(chan string, 10)

	// create pipes to connect the ssh session to our goroutines
	inPipeWr, _ := sess.StdinPipe()
	outPipeRd, _ := sess.StdoutPipe()
	errPipeRd, _ := sess.StderrPipe()

	// fire up the forwarders
	go forwardReaderToChan(outPipeRd, outChan)
	go forwardReaderToChan(errPipeRd, errChan)
	go forwardWriterToChan(inPipeWr, inChan)

	return Broker{
		inst:       name,
		sshConfig:  &sshConf,
		sshClient:  client,
		sshSession: sess,
		stdin:      inChan,
		stdout:     outChan,
		stderr:     errChan,
	}
}

// also cleans ansi text to return plain text
func forwardReaderToChan(reader io.Reader, ch chan string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		text := scanner.Text()
		log.Printf("stdin: %q", text)
		nocr := strings.TrimSuffix(text, "\r")
		clean := ansiCleanRe.ReplaceAllString(nocr, "")
		ch <- clean
	}
	if scanner.Err() != nil {
		log.Fatalf("Error while reading from ssh: %s", scanner.Err())
	}
}

func forwardWriterToChan(writer io.WriteCloser, ch chan string) {
	for {
		select {
		case text := <-ch:
			if strings.HasSuffix(text, "\n") {
				writer.Write([]byte(text))
			} else {
				writer.Write([]byte(text + "\r\n"))
			}
		}
	}
}

// Name returns the name of the broker as set in NewBroker.
func (b Broker) Name() string {
	return b.inst
}

func (b Broker) Send(evt hal.Evt) {
	lines := strings.Split(evt.Body, "\n")
	for _, line := range lines {
		b.stdin <- line
	}
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
	log.Printf("listening on sshchat...")

	for {
		select {
		case msg := <-b.stdout:
			now := time.Now()

			matches := parseMsgRe.FindStringSubmatch(msg)
			if len(matches) != 3 {
				log.Printf("Unable to parse message %q", msg)
				continue
			}

			user := matches[1]
			body := matches[2]

			e := hal.Evt{
				ID:       fmt.Sprintf("%d.%06d", now.Unix(), now.UnixNano()),
				User:     user,
				UserId:   user,
				Room:     "lobby",
				RoomId:   "lobby",
				Body:     body,
				Time:     now,
				Broker:   b,
				IsChat:   true,
				Original: &msg,
			}

			out <- &e
		case msg := <-b.stderr:
			log.Printf("Server stderr: %q", msg)
		}
	}

	log.Printf("no longer listening on sshchat...")
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

// requires there to be an unencrypted ssh key in ~/.ssh/insecure-key
// TODO: replace hard-coded values
func (c Config) loadSSHKey() []ssh.AuthMethod {
	kf := c.SSHKeyFile
	keyText, err := ioutil.ReadFile(kf)
	if err != nil {
		log.Fatalf("Could not read ssh private key file %s: %s", kf, err)
	}

	signer, err := ssh.ParsePrivateKey(keyText)
	if err != nil {
		log.Fatalf("Failed to parse ssh private key file %s: %s", kf, err)
	}

	return []ssh.AuthMethod{ssh.PublicKeys(signer)}
}
