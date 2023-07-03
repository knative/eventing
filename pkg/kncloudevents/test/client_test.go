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

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/cloudevents/sdk-go/v2/test"
	"github.com/stretchr/testify/require"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func Test_inMemoryRequest_SentEvents(t *testing.T) {
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
			name: "Should count received events correclty",
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
			req, err := imClient.NewRequest(ctx, duckv1.Addressable{
				URL: apis.HTTP("foo.bar"),
			})
			require.Nil(t, err)

			for _, event := range tt.eventsToSend {
				message := binding.ToMessage(&event)
				defer message.Finish(nil)

				err = http.WriteRequest(ctx, message, req.HTTPRequest())
				require.Nil(t, err)

				_, err = req.Send()
				require.Nil(t, err)
			}

			imRequest := req.(*inMemoryRequest)
			require.Equal(t, len(imRequest.SentEvents()), tt.wantReceivedEvents)
		})
	}
}

func TestClient_SentEvents(t *testing.T) {
	client := NewClient()
	ctx := context.TODO()
	target := duckv1.Addressable{
		URL: apis.HTTP("foo.bar"),
	}

	// without any send requests it should be 0
	require.Len(t, client.SentEvents(), 0)

	req1, err := client.NewRequest(ctx, target)
	require.Nil(t, err)
	require.Len(t, client.SentEvents(), 0)

	// now bind one event (not sent)
	req1.BindEvent(ctx, test.FullEvent())
	require.Len(t, client.SentEvents(), 0)

	_, err = req1.Send()
	require.Nil(t, err)
	require.Len(t, client.SentEvents(), 1)

	// do a 2nd request
	req2, err := client.NewRequest(ctx, target)
	require.Nil(t, err)
	req2.BindEvent(ctx, test.FullEvent())
	require.Len(t, client.SentEvents(), 1)

	// send 2nd request
	_, err = req2.Send()
	require.Nil(t, err)
	require.Len(t, client.SentEvents(), 2)
}
