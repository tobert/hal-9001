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
	User    string
	Channel string
	stdin   chan string
	stdout  chan string
}

func (c *Config) NewBroker(name string) *Broker {
	user := os.Getenv("USER")
	if user == "" {
		user = "testuser"
	}

	out := Broker{
		User:    user,
		Channel: name,
		stdin:   make(chan string, 1000),
		stdout:  make(chan string, 1000),
	}

	return &out
}

func (cb *Broker) Name() string {
	return cb.Channel
}

func (cb *Broker) Send(e hal.Evt) {
	cb.stdout <- fmt.Sprintf("%s/%s: %s\n", e.From, e.Channel, e.Body)
}

func (cb *Broker) Stream(out chan *hal.Evt) {
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
				From:      cb.User,
				Channel:   cb.Channel,
				Body:      evt,
				Time:      time.Now(),
				Broker:    cb,
				IsGeneric: true,
				Original:  &evt,
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
