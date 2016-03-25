package hal

// Broker is an instance of a broker that can send/receive events.
type Broker interface {
	Name() string
	Send(Evt)
	RoomIdToName(string) string
	RoomNameToId(string) string
	UserIdToName(string) string
	UserNameToId(string) string
	Stream(out chan *Evt)
}

// BrokerConfig is used to create named instances of brokers using NewBroker()
type BrokerConfig interface {
	NewBroker(name string) Broker
}

// TODO: consider reverting all this Broker stack stuff now that forwarding
// has proven to be a bad idea (that led to a better one)

// Brokers is a stack of brokers.
type Brokers []Broker

// Has returns true if the handle's stack contains a broker with the provided name.
func (bs Brokers) Has(name string) bool {
	for _, b := range bs {
		if b.Name() == name {
			return true
		}
	}

	return false
}

// Push puts a broker at the top of the stack.
func (bs Brokers) Push(b Broker) {
	bs = append(bs, b)
}

// Pop removes the broker from the top of the stack and returns it.
func (bs Brokers) Pop() Broker {
	if len(bs) == 0 {
		panic("Pop() called on empty broker list.")
	}

	out := bs[len(bs)-1]

	bs = bs[0 : len(bs)-2]

	return out
}

// First returns the first broker on the stack.
func (bs Brokers) First() Broker {
	if len(bs) == 0 {
		panic("First() called on empty broker list.")
	}

	return bs[0]
}

// Last returns the last broker on the stack.
func (bs Brokers) Last() Broker {
	if len(bs) == 0 {
		panic("Last() called on empty broker list.")
	}

	return bs[len(bs)-1]
}

// Previous finds the provided broker by matching its name against
// an item in the broker list and returns the item before it in that list.
func (bs Brokers) Previous(current Broker) Broker {
	if len(bs) == 0 {
		panic("Previous() called on empty broker list.")
	}
	if len(bs) == 1 {
		panic("Previous() called on a broker list with only one item.")
	}

	for i, b := range bs {
		if b.Name() == current.Name() {
			if i == 0 {
				panic("Previous() called on the first entry in the broker list.")
			}

			return bs[i-1]
		}
	}

	panic("Couldn't find a previous broker.")
}

// Clone returns a shallow-cloned Brokers list.
func (bs Brokers) Clone() Brokers {
	clone := make(Brokers, len(bs))
	copy(clone, bs)
	return clone
}
