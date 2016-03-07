package hal

import (
	"fmt"
	"log"
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

// forward from one (go) channel to another
// TODO: figure out if this needs to check for closed channels, etc.
func forward(from, to chan *Evt) {
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
	go forward(b2r, r.in)

	r.brokers[b.Name()] = b
}

func (r *RouterCTX) GetBroker(name string) Broker {
	if broker, exists := r.brokers[name]; exists {
		return broker
	}

	return nil
}

// Route is the main method for the router. It blocks and should be run in a goroutine
// exactly once.
func (r *RouterCTX) Route() {
	for {
		select {
		case evt := <-r.in:
			if evt.Broker == nil {
				panic("BUG: received event with nil Broker. This breaks all the things!")
			}

			go r.processEvent(evt)
		}
	}
}

// processEvent processes one event and is intended to run in a goroutine.
func (r *RouterCTX) processEvent(evt *Evt) {
	var pname string // must be in the recovery handler's scope

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
		}
	}()

	for _, inst := range instances {
		ibname := inst.Broker.Name()
		pname = inst.Plugin.Name // recovery handler ^ will pick this up in a panic

		// a plugin instance matches on broker, channel, and regex
		// first, check if the instance is attached to a specific broker or generic
		if ibname != evt.Broker.Name() && ibname != gBroker.Name() {
			continue
		}

		// if this is a generic broker instance and the event is marked as not
		// generic, skip it
		if ibname == gBroker.Name() && !evt.IsGeneric {
			continue
		}

		// check if it's the correct channel
		if evt.Channel != inst.Channel {
			continue
		}

		// finally, check message text against the regex
		if inst.Regex != "" && inst.regex.MatchString(evt.Body) {
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
		}
	}
}
