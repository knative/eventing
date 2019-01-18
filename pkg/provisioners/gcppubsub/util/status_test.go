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

package util

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
)

func TestIsEmpty(t *testing.T) {
	testCases := map[string]struct {
		pcs      *GcpPubSubChannelStatus
		expected bool
	}{
		"secret": {
			pcs: &GcpPubSubChannelStatus{
				Secret: &v1.ObjectReference{
					Name: "some-name",
				},
			},
			expected: false,
		},
		"secretKey": {
			pcs: &GcpPubSubChannelStatus{
				SecretKey: "some-key",
			},
			expected: false,
		},
		"gcpProject": {
			pcs: &GcpPubSubChannelStatus{
				GCPProject: "some-project",
			},
			expected: false,
		},
		"topic": {
			pcs: &GcpPubSubChannelStatus{
				Topic: "some-topic",
			},
			expected: false,
		},
		"subscriptions": {
			pcs: &GcpPubSubChannelStatus{
				Subscriptions: []GcpPubSubSubscriptionStatus{
					{
						Subscription: "some-subscription",
					},
				},
			},
			expected: false,
		},
		"empty": {
			pcs:      &GcpPubSubChannelStatus{},
			expected: true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			actual := tc.pcs.IsEmpty()
			if actual != tc.expected {
				t.Errorf("Expected %v. Actual %v", tc.expected, actual)
			}
		})
	}
}

func TestReadInternalStatus(t *testing.T) {
	testCases := map[string]struct {
		internal *runtime.RawExtension
		expected *GcpPubSubChannelStatus
		err      bool
	}{
		"nil": {
			internal: nil,
			expected: &GcpPubSubChannelStatus{},
			err:      false,
		},
		"empty": {
			internal: &runtime.RawExtension{
				Raw: []byte{},
			},
			expected: &GcpPubSubChannelStatus{},
			err:      false,
		},
		"can't unmarshal": {
			internal: &runtime.RawExtension{
				// The topic field is a string, so this will have an error during unmarshal.
				Raw: []byte(`{"topic": 123}`),
			},
			err: true,
		},
		"success": {
			internal: &runtime.RawExtension{
				Raw: []byte(`{"topic":"gcp-topic"}`),
			},
			expected: &GcpPubSubChannelStatus{
				Topic: "gcp-topic",
			},
			err: false,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			c := &v1alpha1.Channel{}
			c.Status.Internal = tc.internal
			pcs, err := GetInternalStatus(c)
			if tc.err != (err != nil) {
				t.Fatalf("Unexpected error. Expected %v. Actual %v", tc.err, err)
			}
			if diff := cmp.Diff(tc.expected, pcs); diff != "" {
				t.Fatalf("Unexpected difference (-want +got): %v", diff)
			}
		})
	}
}

func TestInternalStatusRoundTrip(t *testing.T) {
	testCases := map[string]struct {
		pcs *GcpPubSubChannelStatus
	}{
		"all fields set": {
			pcs: &GcpPubSubChannelStatus{
				Secret: &v1.ObjectReference{
					Name: "some-name",
				},
				SecretKey:  "some-key",
				GCPProject: "some-project",
				Topic:      "some-topic",
				Subscriptions: []GcpPubSubSubscriptionStatus{
					{
						Subscription: "some-subscription",
					},
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			c := &v1alpha1.Channel{}
			err := SetInternalStatus(context.Background(), c, tc.pcs)
			if err != nil {
				t.Errorf("Unexpected error saving internal status. %v", err)
			}
			pcs, err := GetInternalStatus(c)
			if err != nil {
				t.Errorf("Unexpected error reading internal status. %v", err)
			}
			if diff := cmp.Diff(tc.pcs, pcs); diff != "" {
				t.Errorf("Unexpected parsed internal status (-want +got): %v", diff)
			}
		})
	}
}
