package hal

// genericBroker receives copies of messages from other brokers.
// Plugins targeting this broker can be generic and do not need
// to understand anything about the upstream broker's events.
// This type is exposed so plugins can be tied to it.
type GenericBroker struct{}

var gBroker GenericBroker

// Send on generic broker relies on the event having a broker set
// that is not the generic broker (which has nowhere to send things).
// It's just a proxy that calls evt.Broker.Send().
func (gb GenericBroker) Send(evt Evt) {
	evt.Broker.Send(evt)
}

// GetGenericBroker returns the singleton handle for the generic broker.
func GetGenericBroker() GenericBroker {
	return gBroker
}

// Name returns "generic"
func (gb GenericBroker) Name() string {
	return "generic"
}

// Stream does nothing and is present to fulfill the interface.
// It blocks forever if called.
func (gb GenericBroker) Stream(out chan *Evt) {
	select {}
}

// required by interface
func (gb GenericBroker) RoomIdToName(in string) string { return in }
func (gb GenericBroker) RoomNameToId(in string) string { return in }
func (gb GenericBroker) UserIdToName(in string) string { return in }
func (gb GenericBroker) UserNameToId(in string) string { return in }
