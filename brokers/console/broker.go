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
	cb.stdout <- fmt.Sprintf("%s/%s: %s\n", e.User, e.Room, e.Body)
}

func (cb Broker) SendTable(e hal.Evt, hdr []string, rows [][]string) {
	cb.stdout <- hal.Utf8Table(hdr, rows)
}

// SimpleStdin will loop forever reading stdin and publish each line
// as an event in the console broker.
func (cb Broker) SimpleStdin() {
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
}

func (cb Broker) Line(line string) {
	cb.stdin <- line
}

func (cb Broker) Stream(out chan *hal.Evt) {
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
				Broker:   cb,
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
