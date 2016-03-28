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
