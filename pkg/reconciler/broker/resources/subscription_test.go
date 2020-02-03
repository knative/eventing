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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestMakeSubscription(t *testing.T) {
	testCases := map[string]struct {
		channelable duckv1alpha1.Channelable
	}{
		"InMemoryChannel": {
			channelable: duckv1alpha1.Channelable{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "messaging.knative.dev/v1alpha1",
					Kind:       "InMemoryChannel",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "in-memory-channel",
				},
			},
		},
		"KafkaChannel": {
			channelable: duckv1alpha1.Channelable{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "messaging.knative.dev/v1alpha1",
					Kind:       "KafkaChannel",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "kafka-channel",
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			b := &v1alpha1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "brokers-namespace",
					Name:      "my-broker",
					UID:       "1234",
				},
			}
			svc := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-svc",
				},
			}
			sub := MakeSubscription(b, &tc.channelable, svc)

			if ns := sub.Namespace; ns != b.Namespace {
				t.Errorf("Expected namespace %q, actually %q", b.Namespace, ns)
			}
			if !metav1.IsControlledBy(sub, b) {
				t.Errorf("Expected sub to be controlled by the broker")
			}
			expectedChannel := corev1.ObjectReference{
				APIVersion: tc.channelable.APIVersion,
				Kind:       tc.channelable.Kind,
				Name:       tc.channelable.Name,
			}
			if ch := sub.Spec.Channel; ch != expectedChannel {
				t.Errorf("Expected spec.channel %q, actually %q", expectedChannel, ch)
			}
			expectedSubscriber := duckv1.KReference{
				APIVersion: "v1",
				Kind:       "Service",
				Name:       svc.Name,
				Namespace:  svc.Namespace,
			}
			if subscriber := *sub.Spec.Subscriber.Ref; subscriber != expectedSubscriber {
				t.Errorf("Expected spec.subscriber.ref %q, actually %q", expectedSubscriber, subscriber)
			}
		})
	}
}
