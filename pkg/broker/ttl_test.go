package broker

import (
	"context"
	cloudevents "github.com/cloudevents/sdk-go"
	"go.uber.org/zap"
	"testing"
)

func TestTTLDefaulter(t *testing.T) {
	defaultTTL := int32(10)

	defaulter := TTLDefaulter(zap.NewNop(), defaultTTL)
	ctx := context.TODO()

	tests := map[string]struct {
		event cloudevents.Event
		want  int32
	}{
		// TODO: Add test cases.
		"happy empty": {
			event: cloudevents.NewEvent(),
			want:  defaultTTL,
		},
		"existing ttl of 10": {
			event: func() cloudevents.Event {
				event := cloudevents.NewEvent()
				_ = SetTTL(event.Context, 10)
				return event
			}(),
			want: 9,
		},
		"existing ttl of 1": {
			event: func() cloudevents.Event {
				event := cloudevents.NewEvent()
				_ = SetTTL(event.Context, 1)
				return event
			}(),
			want: 0,
		},
		"existing invalid ttl of 'XYZ'": {
			event: func() cloudevents.Event {
				event := cloudevents.NewEvent()
				event.SetExtension(TTLAttribute, "XYZ")
				return event
			}(),
			want: defaultTTL,
		},
		"existing ttl of 0": {
			event: func() cloudevents.Event {
				event := cloudevents.NewEvent()
				_ = SetTTL(event.Context, 0)
				return event
			}(),
			want: 0,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			event := defaulter(ctx, tc.event)
			got, err := GetTTL(event.Context)
			if err != nil {
				t.Error(err)
			}
			if got != tc.want {
				t.Errorf("Unexpected TTL, wanted %d, got %d", tc.want, got)
			}
		})
	}
}
