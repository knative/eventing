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

package resources

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	duckeventingv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"

	_ "knative.dev/pkg/system/testing"
)

func TestMakeFilterDeployment(t *testing.T) {
	testCases := map[string]struct {
		trigger       *v1alpha1.Trigger
		brokerTrigger *corev1.ObjectReference
		brokerIngress *corev1.ObjectReference
		delivery      *duckeventingv1alpha1.DeliverySpec
		uri           *url.URL
		want          []byte
	}{
		"happy": {
			trigger: &v1alpha1.Trigger{
				ObjectMeta: v1.ObjectMeta{
					Name:      "happy",
					Namespace: "Bar",
					UID:       "abc-uid",
				},
				Spec: v1alpha1.TriggerSpec{
					Broker: "broker",
				},
			},
			brokerTrigger: &corev1.ObjectReference{
				Kind:      "Foo",
				Namespace: "Bar",
				Name:      "Baz",
			},
			brokerIngress: &corev1.ObjectReference{
				Kind:      "Aoo",
				Namespace: "Bar",
				Name:      "Caz",
			},
			uri: func() *url.URL {
				u, _ := url.Parse("http://example.com/uid")
				return u
			}(),
			delivery: &duckeventingv1alpha1.DeliverySpec{
				DeadLetterSink: &duckv1.Destination{
					URI: &apis.URL{Host: "deadlettersink.example.com"},
				},
			},
			want: []byte(`{
  "metadata": {
    "name": "broker-happy-abc-uid",
    "namespace": "Bar",
    "creationTimestamp": null,
    "labels": {
      "eventing.knative.dev/broker": "broker",
      "eventing.knative.dev/trigger": "happy"
    },
    "ownerReferences": [
      {
        "apiVersion": "eventing.knative.dev/v1alpha1",
        "kind": "Trigger",
        "name": "happy",
        "uid": "abc-uid",
        "controller": true,
        "blockOwnerDeletion": true
      }
    ]
  },
  "spec": {
    "channel": {
      "kind": "Foo",
      "name": "Baz"
    },
    "subscriber": {
      "uri": "http://example.com/uid"
    },
    "reply": {
      "ref": {
        "kind": "Aoo",
        "name": "Caz"
      }
    },
    "delivery": {
      "deadLetterSink": {
        "uri": "//deadlettersink.example.com"
      }
    }
  },
  "status": {
    "physicalSubscription": {}
  }
}`),
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			dep := NewSubscription(tc.trigger, tc.brokerTrigger, tc.brokerIngress, tc.uri, tc.delivery)

			got, err := json.MarshalIndent(dep, "", "  ")
			if err != nil {
				t.Errorf("failed to marshal, %s", err)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Log(string(got))
				t.Errorf("unexpected (-want, +got) = %v", diff)
			}
		})
	}
}

func TestSubscriptionLabels(t *testing.T) {
	testCases := map[string]struct {
		trigger *v1alpha1.Trigger
		want    map[string]string
	}{
		"happy trigger": {
			trigger: &v1alpha1.Trigger{
				TypeMeta: v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{
					Name: "happy",
				},
				Spec: v1alpha1.TriggerSpec{
					Broker: "brokerName",
				},
				Status: v1alpha1.TriggerStatus{},
			},
			want: map[string]string{
				"eventing.knative.dev/broker":  "brokerName",
				"eventing.knative.dev/trigger": "happy",
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			got := SubscriptionLabels(tc.trigger)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("unexpected labels (-want, +got) = %v", diff)
			}
		})
	}
}
