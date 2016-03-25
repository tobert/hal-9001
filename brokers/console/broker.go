package console

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/netflix/hal-9001/hal"
)

type Config struct{}

type Broker struct {
	User   string
	Room   string
	stdin  chan string
	stdout chan string
}

func (c Config) NewBroker(name string) Broker {
	user := os.Getenv("USER")
	if user == "" {
		user = "testuser"
	}

	out := Broker{
		User:   user,
		Room:   name,
		stdin:  make(chan string, 1000),
		stdout: make(chan string, 1000),
	}

	return out
}

func (cb Broker) Name() string {
	return cb.Room
}

func (cb Broker) Send(e hal.Evt) {
	// sender may or may not have specified the broker, make sure
	// this one is the last on the stack and if not, add it
	if e.BrokerName() != cb.Room {
		e.Brokers.Push(cb)
	}

	cb.stdout <- fmt.Sprintf("%s/%s: %s\n", e.User, e.Room, e.Body)
}

func (cb Broker) Stream(out chan *hal.Evt) {
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Text()

			if err := scanner.Err(); err != nil {
				log.Fatalf("Failed while reading from stdin: %s\n", err)
			}

			// ignore empty lines
			if len(line) == 0 {
				continue
			}

			cb.stdin <- line
		}
	}()

	for {
		select {
		case evt := <-cb.stdin:
			e := hal.Evt{
				User:     cb.User,
				UserId:   cb.User,
				Room:     cb.Room,
				RoomId:   cb.Room,
				Body:     evt,
				Time:     time.Now(),
				Brokers:  hal.Brokers{cb},
				Original: &evt,
			}

			out <- &e

		case txt := <-cb.stdout:
			// events from the Reply() method go through a go channel
			_, err := os.Stdout.WriteString(txt)
			if err != nil {
				log.Fatalf("Could not write to stdout: %s\n", err)
			}
		}
	}
}

// required by interface
func (b Broker) RoomIdToName(in string) string { return in }
func (b Broker) RoomNameToId(in string) string { return in }
func (b Broker) UserIdToName(in string) string { return in }
func (b Broker) UserNameToId(in string) string { return in }
