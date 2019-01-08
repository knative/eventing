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
		pbs      *GcpPubSubChannelStatus
		expected bool
	}{
		"secret": {
			pbs: &GcpPubSubChannelStatus{
				Secret: &v1.ObjectReference{
					Name: "some-name",
				},
			},
			expected: false,
		},
		"secretKey": {
			pbs: &GcpPubSubChannelStatus{
				SecretKey: "some-key",
			},
			expected: false,
		},
		"gcpProject": {
			pbs: &GcpPubSubChannelStatus{
				GCPProject: "some-project",
			},
			expected: false,
		},
		"topic": {
			pbs: &GcpPubSubChannelStatus{
				Topic: "some-topic",
			},
			expected: false,
		},
		"subscriptions": {
			pbs: &GcpPubSubChannelStatus{
				Subscriptions: []GcpPubSubSubscriptionStatus{
					{
						Subscription: "some-subscription",
					},
				},
			},
			expected: false,
		},
		"empty": {
			pbs:      &GcpPubSubChannelStatus{},
			expected: true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			actual := tc.pbs.IsEmpty()
			if actual != tc.expected {
				t.Errorf("Expected %v. Actual %v", tc.expected, actual)
			}
		})
	}
}

func TestReadRawStatus(t *testing.T) {
	testCases := map[string]struct {
		raw      *runtime.RawExtension
		expected *GcpPubSubChannelStatus
		err      bool
	}{
		"nil": {
			raw:      nil,
			expected: &GcpPubSubChannelStatus{},
			err:      false,
		},
		"empty": {
			raw: &runtime.RawExtension{
				Raw: []byte{},
			},
			expected: &GcpPubSubChannelStatus{},
			err:      false,
		},
		"can't unmarshal": {
			raw: &runtime.RawExtension{
				// The topic field is a string, so this will have an error during unmarshal.
				Raw: []byte(`{"topic": 123}`),
			},
			err: true,
		},
		"success": {
			raw: &runtime.RawExtension{
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
			c.Status.Raw = tc.raw
			pbs, err := ReadRawStatus(context.Background(), c)
			if tc.err != (err != nil) {
				t.Fatalf("Unexpected error. Expected %v. Actual %v", tc.err, err)
			}
			if diff := cmp.Diff(tc.expected, pbs); diff != "" {
				t.Fatalf("Unexpected difference (-want +got): %v", diff)
			}
		})
	}
}

func TestRawStatusRoundTrip(t *testing.T) {
	testCases := map[string]struct {
		pbs *GcpPubSubChannelStatus
	}{
		"all fields set": {
			pbs: &GcpPubSubChannelStatus{
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
			err := SaveRawStatus(context.Background(), c, tc.pbs)
			if err != nil {
				t.Errorf("Unexpected error saving raw status. %v", err)
			}
			pbs, err := ReadRawStatus(context.Background(), c)
			if err != nil {
				t.Errorf("Unexpected error reading raw status. %v", err)
			}
			if diff := cmp.Diff(tc.pbs, pbs); diff != "" {
				t.Errorf("Unexpected parsed raw status (-want +got): %v", diff)
			}
		})
	}
}
