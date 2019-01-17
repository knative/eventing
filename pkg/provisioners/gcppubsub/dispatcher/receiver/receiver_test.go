/*
 * Copyright 2018 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package receiver

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/knative/eventing/pkg/provisioners/gcppubsub/util"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/knative/eventing/pkg/provisioners/gcppubsub/util/fakepubsub"
	"go.uber.org/zap"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/knative/eventing/pkg/provisioners/gcppubsub/util/testcreds"
)

const (
	validMessage = `{
    "cloudEventsVersion" : "0.1",
    "eventType" : "com.example.someevent",
    "eventTypeVersion" : "1.0",
    "source" : "/mycontext",
    "eventID" : "A234-1234-1234",
    "eventTime" : "2018-04-05T17:31:00Z",
    "extensions" : {
      "comExampleExtension" : "value"
    },
    "contentType" : "text/xml",
    "data" : "<much wow=\"xml\"/>"
}`
)

func init() {
	// Add types to scheme.
	eventingv1alpha1.AddToScheme(scheme.Scheme)
}

func TestReceiver(t *testing.T) {
	testCases := map[string]struct {
		initialState []runtime.Object
		pubSubData   fakepubsub.CreatorData
		expectedErr  bool
	}{
		"can't get channel": {
			initialState: []runtime.Object{
				testcreds.MakeSecretWithInvalidCreds(),
			},
			expectedErr: true,
		},
		"can't read status": {
			initialState: []runtime.Object{
				testcreds.MakeSecretWithInvalidCreds(),
				makeChannelWithBadStatus(),
			},
			expectedErr: true,
		},
		"blank status": {
			initialState: []runtime.Object{
				testcreds.MakeSecretWithInvalidCreds(),
				makeChannelWithBlankStatus(),
			},
			expectedErr: true,
		},
		"credential fails": {
			initialState: []runtime.Object{
				testcreds.MakeSecretWithInvalidCreds(),
				makeChannel(),
			},
			expectedErr: true,
		},
		"PubSub client fails": {
			initialState: []runtime.Object{
				testcreds.MakeSecretWithCreds(),
				makeChannel(),
			},
			pubSubData: fakepubsub.CreatorData{
				ClientCreateErr: errors.New("testInducedError"),
			},
			expectedErr: true,
		},
		"Publish fails": {
			initialState: []runtime.Object{
				testcreds.MakeSecretWithCreds(),
				makeChannel(),
			},
			pubSubData: fakepubsub.CreatorData{
				ClientData: fakepubsub.ClientData{
					TopicData: fakepubsub.TopicData{
						Publish: fakepubsub.PublishResultData{
							Err: errors.New("testInducedError"),
						},
					},
				},
			},
			expectedErr: true,
		},
		"Publish succeeds": {
			initialState: []runtime.Object{
				testcreds.MakeSecretWithCreds(),
				makeChannel(),
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			mr, _ := New(
				zap.NewNop(),
				fake.NewFakeClient(tc.initialState...),
				fakepubsub.Creator(tc.pubSubData))
			resp := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/", strings.NewReader(validMessage))
			req.Host = "test-channel.test-namespace.channels.cluster.local"
			mr.newMessageReceiver().HandleRequest(resp, req)
			if tc.expectedErr {
				if resp.Result().StatusCode >= 200 && resp.Result().StatusCode < 300 {
					t.Errorf("Expected an error. Actual: %v", resp.Result())
				}
			} else {
				if resp.Result().StatusCode < 200 || resp.Result().StatusCode >= 300 {
					t.Errorf("Expected success. Actual: %v", resp.Result())
				}
			}
		})
	}
}

func makeChannel() *eventingv1alpha1.Channel {
	c := &eventingv1alpha1.Channel{
		TypeMeta: v1.TypeMeta{
			APIVersion: "eventing.knative.dev/v1alpha1",
			Kind:       "Channel",
		},
		ObjectMeta: v1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-channel",
		},
	}
	pcs := &util.GcpPubSubChannelStatus{
		GCPProject: "project",
		Secret:     testcreds.Secret,
		SecretKey:  testcreds.SecretKey,
	}
	if err := util.SetInternalStatus(context.Background(), c, pcs); err != nil {
		panic(err)
	}
	return c
}

func makeChannelWithBlankStatus() *eventingv1alpha1.Channel {
	c := makeChannel()
	c.Status = eventingv1alpha1.ChannelStatus{}
	return c
}

func makeChannelWithBadStatus() *eventingv1alpha1.Channel {
	c := makeChannel()
	c.Status.Internal = &runtime.RawExtension{
		// SecretKey must be a string, not an integer, so this will fail during json.Unmarshal.
		Raw: []byte(`{"secretKey": 123}`),
	}
	return c
}
