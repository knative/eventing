package client

import (
	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"github.com/google/uuid"
	"time"
)

// EventDefaulter is the function signature for extensions that are able
// to perform event defaulting.
type EventDefaulter func(event cloudevents.Event) cloudevents.Event

// DefaultIDToUUIDIfNotSet will inspect the provided event and assign a UUID to
// context.ID if it is found to be empty.
func DefaultIDToUUIDIfNotSet(event cloudevents.Event) cloudevents.Event {
	if event.Context != nil {
		switch event.Context.GetSpecVersion() {
		case cloudevents.CloudEventsVersionV01:
			ec := event.Context.AsV01()
			if ec.EventID == "" {
				ec.EventID = uuid.New().String()
				event.Context = ec
			}
		case cloudevents.CloudEventsVersionV02:
			ec := event.Context.AsV02()
			if ec.ID == "" {
				ec.ID = uuid.New().String()
				event.Context = ec
			}
		case cloudevents.CloudEventsVersionV03:
			ec := event.Context.AsV03()
			if ec.ID == "" {
				ec.ID = uuid.New().String()
				event.Context = ec
			}
		}
	}
	return event
}

// DefaultTimeToNowIfNotSet will inspect the provided event and assign a new
// Timestamp to context.Time if it is found to be nil or zero.
func DefaultTimeToNowIfNotSet(event cloudevents.Event) cloudevents.Event {
	if event.Context != nil {
		switch event.Context.GetSpecVersion() {
		case cloudevents.CloudEventsVersionV01:
			ec := event.Context.AsV01()
			if ec.EventTime == nil || ec.EventTime.IsZero() {
				ec.EventTime = &types.Timestamp{Time: time.Now()}
				event.Context = ec
			}
		case cloudevents.CloudEventsVersionV02:
			ec := event.Context.AsV02()
			if ec.Time == nil || ec.Time.IsZero() {
				ec.Time = &types.Timestamp{Time: time.Now()}
				event.Context = ec
			}
		case cloudevents.CloudEventsVersionV03:
			ec := event.Context.AsV03()
			if ec.Time == nil || ec.Time.IsZero() {
				ec.Time = &types.Timestamp{Time: time.Now()}
				event.Context = ec
			}
		}
	}
	return event
}
