/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package broker

import (
	"context"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"
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
