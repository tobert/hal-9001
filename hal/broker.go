package hal

// Broker is an instance of a broker that can send/receive events.
type Broker interface {
	// the text name of the broker, arbitrary, but usually "slack" or "cli"
	Name() string
	Send(evt Evt)
	SendTable(evt Evt, header []string, rows [][]string)
	RoomIdToName(id string) (name string)
	RoomNameToId(name string) (id string)
	UserIdToName(id string) (name string)
	UserNameToId(name string) (id string)
	Stream(out chan *Evt)
}
