package wires

type EventReceiver struct {
	receiveFunc func()
}
