package cloudevents

// EventResponse represents the canonical representation of a Response to a
// CloudEvent from a receiver.
type EventResponse struct {
	Status int
	Event  *Event
	Reason string
}

func (e *EventResponse) RespondWith(status int, event *Event) {
	if e == nil {
		// if nil, response not supported
		return
	}
	e.Status = status
	if event != nil {
		e.Event = event
	}
}

func (e *EventResponse) Error(status int, reason string) {
	if e == nil {
		// if nil, response not supported
		return
	}
	e.Status = status
	e.Reason = reason
}
