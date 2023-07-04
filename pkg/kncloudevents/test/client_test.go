/*
Copyright 2023 The Knative Authors

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

package test

import (
	"context"
	"testing"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/test"
	"github.com/stretchr/testify/require"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func Test_Client_SentEvents(t *testing.T) {
	tests := []struct {
		name               string
		eventsToSend       []event.Event
		wantReceivedEvents int
	}{
		{
			name:               "No error on no events to send",
			eventsToSend:       nil,
			wantReceivedEvents: 0,
		},
		{
			name: "Should count received events correctly",
			eventsToSend: []event.Event{
				test.FullEvent(),
				test.FullEvent(),
			},
			wantReceivedEvents: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()

			imClient := NewClient()

			for _, event := range tt.eventsToSend {
				req, err := kncloudevents.NewRequest(ctx, duckv1.Addressable{
					URL: apis.HTTP("foo.bar"),
				})
				require.Nil(t, err)

				err = req.BindEvent(ctx, event)
				require.Nil(t, err)

				_, err = imClient.Send(req)
				require.Nil(t, err)
			}

			require.Equal(t, len(imClient.SentEvents()), tt.wantReceivedEvents)
		})
	}
}
