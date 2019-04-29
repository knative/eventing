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
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/eventing/contrib/gcppubsub/pkg/util"
	"github.com/knative/eventing/contrib/gcppubsub/pkg/util/fakepubsub"
	"github.com/knative/eventing/contrib/gcppubsub/pkg/util/testcreds"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners"
	controllertesting "github.com/knative/eventing/pkg/reconciler/testing"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
	ccpName            = "gcp-pubsub"
	listChannelsFailed = "Failed to list channels"
	hostname           = "a.b.c.d"
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
				makeChannel(withBadStatus()),
			},
			expectedErr: true,
		},
		"blank status": {
			initialState: []runtime.Object{
				testcreds.MakeSecretWithInvalidCreds(),
				makeChannel(withBlankStatus()),
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
				makeChannel(withStatusReady(hostname)),
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			mr, _, err := New(
				zap.NewNop(),
				fake.NewFakeClient(tc.initialState...),
				fakepubsub.Creator(tc.pubSubData))
			if err != nil {
				t.Fatalf("Error when creating a New receiver. Error:%s", err)
			}
			mr.setHostToChannelMap(map[string]provisioners.ChannelReference{})
			resp := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/", strings.NewReader(validMessage))
			req.Host = hostname
			receiver, err := mr.newMessageReceiver()
			if err != nil {
				t.Fatalf("Error when creating a new message receiver. Error:%s", err)
			}
			mr.UpdateHostToChannelMap(context.TODO())
			receiver.HandleRequest(resp, req)
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

func TestUpdateHostToChannelMap(t *testing.T) {
	testCases := []struct {
		name           string
		initialState   []runtime.Object
		expectedMap    map[string]provisioners.ChannelReference
		expectedErrMsg string
		mocks          controllertesting.Mocks
	}{
		{
			name: "client.List() channels fails.",
			initialState: []runtime.Object{
				makeChannel(withStatusReady(hostname)),
			},
			expectedErrMsg: listChannelsFailed,
			expectedMap:    map[string]provisioners.ChannelReference{},
			mocks: controllertesting.Mocks{
				MockLists: []controllertesting.MockList{
					func(_ client.Client, _ context.Context, _ *client.ListOptions, _ runtime.Object) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, fmt.Errorf(listChannelsFailed)
					},
				},
			},
		},
		{
			name: "Duplciate hostnames.",
			initialState: []runtime.Object{
				makeChannel(withName("chan1"), withNamespace("ns1"), withStatusReady("host.name")),
				makeChannel(withName("chan2"), withNamespace("ns2"), withStatusReady("host.name")),
			},
			expectedErrMsg: "Duplicate hostName found. Each channel must have a unique host header. HostName:host.name, channel:ns2.chan2, channel:ns1.chan1",
			expectedMap:    map[string]provisioners.ChannelReference{},
		},
		{
			name: "Successfully updated hostToChannelMap.",
			initialState: []runtime.Object{
				makeChannel(withName("chan1"), withNamespace("ns1"), withStatusReady("host.name1")),
				makeChannel(withName("chan2"), withNamespace("ns2"), withStatusReady("host.name2")),
			},
			expectedMap: map[string]provisioners.ChannelReference{
				"host.name1": provisioners.ChannelReference{Name: "chan1", Namespace: "ns1"},
				"host.name2": provisioners.ChannelReference{Name: "chan2", Namespace: "ns2"},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := controllertesting.NewMockClient(fake.NewFakeClient(tc.initialState...), tc.mocks)
			r, _, err := New(zap.NewNop(), c, fakepubsub.Creator(nil))
			if err != nil {
				t.Fatalf("Failed to create receiver.")
			}
			if err := r.UpdateHostToChannelMap(context.Background()); err != nil {
				if diff := cmp.Diff(tc.expectedErrMsg, err.Error()); diff != "" {
					t.Fatalf("Unexpected difference (-want +got): %v", diff)
				}
			} else if tc.expectedErrMsg != "" {
				t.Fatalf("Want error:%s, Got nil", tc.expectedErrMsg)
			}

			if diff := cmp.Diff(tc.expectedMap, r.getHostToChannelMap()); diff != "" {
				t.Fatalf("Unexpected difference (-want +got): %v", diff)
			}
		})
	}
}

type option func(*eventingv1alpha1.Channel)

func withName(name string) option {
	return func(c *eventingv1alpha1.Channel) {
		c.Name = name
	}
}

func withNamespace(ns string) option {
	return func(c *eventingv1alpha1.Channel) {
		c.Namespace = ns
	}
}

func withStatusReady(hn string) option {
	return func(c *eventingv1alpha1.Channel) {
		c.Status.InitializeConditions()
		c.Status.InitializeConditions()
		c.Status.MarkProvisioned()
		c.Status.MarkProvisionerInstalled()
		c.Status.SetAddress(hn)
	}
}

func withBlankStatus() option {
	return func(c *eventingv1alpha1.Channel) {
		c.Status = eventingv1alpha1.ChannelStatus{}
	}
}

func withBadStatus() option {
	return func(c *eventingv1alpha1.Channel) {
		c.Status.Internal = &runtime.RawExtension{
			// SecretKey must be a string, not an integer, so this will fail during json.Unmarshal.
			Raw: []byte(`{"secretKey": 123}`),
		}
	}
}

func makeChannel(opts ...option) *eventingv1alpha1.Channel {
	c := &eventingv1alpha1.Channel{
		TypeMeta: v1.TypeMeta{
			APIVersion: "eventing.knative.dev/v1alpha1",
			Kind:       "Channel",
		},
		ObjectMeta: v1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-channel",
		},
		Spec: eventingv1alpha1.ChannelSpec{
			Provisioner: &corev1.ObjectReference{
				Name: ccpName,
			},
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

	for _, opt := range opts {
		opt(c)
	}
	return c
}
