package hal

/*
 * Copyright 2016 Albert P. Tobey <atobey@netflix.com>
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
	"log"
	"runtime/debug"
	"strings"
	"sync"
)

// RouterCTX holds the router's context, including input/output chans.
type RouterCTX struct {
	brokers map[string]Broker
	in      chan *Evt     // messages from brokers --> plugins
	out     chan *Evt     // messages from plugins --> brokers
	update  chan struct{} // to notify the router that the instance list changed
	mut     sync.Mutex
	init    sync.Once
}

type fwdBroker struct {
	from Broker
	to   Broker
}

var routerSingleton RouterCTX

// Router returns the singleton router context. The router is initialized
// on the first call to this function.
func Router() *RouterCTX {
	routerSingleton.init.Do(func() {
		routerSingleton.in = make(chan *Evt, 1000)
		routerSingleton.out = make(chan *Evt, 1000)
		routerSingleton.update = make(chan struct{}, 1)
		routerSingleton.brokers = make(map[string]Broker)
	})

	return &routerSingleton
}

// forwardChan forwards events from one chan of to another.
// TODO: figure out if this needs to check for closed channels, etc.
func forwardChan(from, to chan *Evt) {
	for {
		select {
		case evt := <-from:
			to <- evt
		}
	}
}

// AddBroker adds a broker to the router and starts forwarding
// events between it and the router.
func (r *RouterCTX) AddBroker(b Broker) {
	r.mut.Lock()
	defer r.mut.Unlock()

	if _, exists := r.brokers[b.Name()]; exists {
		panic(fmt.Sprintf("BUG: broker '%s' added > 1 times.", b.Name()))
	}

	b2r := make(chan *Evt, 1000) // messages from the broker to the router

	// start the broker's event stream
	go b.Stream(b2r)

	// forward events from the broker to the router's input channel
	go forwardChan(b2r, r.in)

	r.brokers[b.Name()] = b
}

// GetBroker retrieves a broker handle by name.
func (r *RouterCTX) GetBroker(name string) Broker {
	r.mut.Lock()
	defer r.mut.Unlock()

	if broker, exists := r.brokers[name]; exists {
		return broker
	}

	return nil
}

// Brokers returns all brokers that have been added to the router.
// The returned list is not in any particular order.
func (r *RouterCTX) Brokers() []Broker {
	r.mut.Lock()
	defer r.mut.Unlock()

	out := make([]Broker, len(r.brokers))
	i := 0
	for _, b := range r.brokers {
		out[i] = b
		i++
	}

	return out
}

// Route is the main method for the router. It blocks and should be run in a
// goroutine exactly once. Running more than one router in the same process
// will result in shenanigans.
func (r *RouterCTX) Route() {
	for {
		select {
		case evt := <-r.in:
			// events are processed concurrently, plugins are not
			go r.processEvent(evt)
		}
	}
}

// processEvent processes one event and is intended to run in a goroutine.
func (r *RouterCTX) processEvent(evt *Evt) {
	var pname string // must be in the recovery handler's scope

	// detect invalid commands & count executions
	var ranPlugins int

	// get a snapshot of the instance list
	// TODO: keep an eye on the cost of copying this list for every message
	pr := PluginRegistry()
	instances := pr.InstanceList()

	// if a plugin panics, catch it & log it
	// TODO: report errors to a channel?
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered panic in plugin %q\n", pname)
			log.Printf("panic: %q", r)
			debug.PrintStack()
		}
	}()

	for _, inst := range instances {
		// the recovery handler will pick this up in a panic to provide
		// the name of the plugin that caused the panic
		pname = inst.Plugin.Name

		// check if it's the correct room
		if evt.RoomId != inst.RoomId {
			continue
		}

		// finally, check message text against the regex
		if inst.Regex == "" || inst.regex.MatchString(evt.Body) {
			// this will copy the struct twice. It's intentional to avoid
			// mutating the evt between calls. The plugin func signature
			// forces the second copy.
			evtcpy := *evt

			// pass the plugin instance pointer to the plugin function so
			// it can access its fields for settings, etc.
			evtcpy.instance = inst

			// call the plugin function
			// this may block other plugins from processing the same event but
			// since it's already in a goroutine, other events won't be blocked
			inst.Func(evtcpy)

			ranPlugins++
		}
	}

	if ranPlugins == 0 && strings.HasPrefix(strings.TrimSpace(evt.Body), "!") {
		mgr, err := pr.GetPlugin("pluginmgr")
		// only proceed if there is no error - bots may choose to exclude pluginmgr
		if strings.HasPrefix(strings.TrimSpace(evt.Body), "!plugin") && err == nil {
			inst := mgr.Instance(evt.RoomId, evt.Broker)
			evtcpy := *evt
			evtcpy.instance = inst
			inst.Func(evtcpy)
		} else {
			evt.Replyf("%q: invalid command (%d plugins were executed for the event).", evt.Body, ranPlugins)
		}
	}
}
