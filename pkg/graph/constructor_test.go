/*
Copyright 2024 The Knative Authors

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

package graph

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	eventingv1beta3 "knative.dev/eventing/pkg/apis/eventing/v1beta3"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	sampleUri, _ = apis.ParseURL("https://knative.dev")
	secondUri, _ = apis.ParseURL("https://google.com")
	thirdUri, _  = apis.ParseURL("https://github.com")
)

func TestAddBroker(t *testing.T) {
	brokerWithEdge := &Vertex{
		self: &duckv1.Destination{
			Ref: &duckv1.KReference{
				Name:       "my-broker",
				Namespace:  "default",
				APIVersion: "eventing.knative.dev/v1",
				Kind:       "Broker",
			},
		},
	}
	destinationWithEdge := &Vertex{
		self: &duckv1.Destination{
			URI: sampleUri,
		},
	}
	brokerWithEdge.AddEdge(destinationWithEdge, &duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       "my-broker",
			Namespace:  "default",
			APIVersion: "eventing.knative.dev/v1",
			Kind:       "Broker",
		},
	}, NoTransform{}, true)
	tests := []struct {
		name     string
		broker   eventingv1.Broker
		expected map[comparableDestination]*Vertex
	}{
		{
			name: "no DLS",
			broker: eventingv1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-broker",
					Namespace: "default",
				},
			},
			expected: map[comparableDestination]*Vertex{
				{
					Ref: duckv1.KReference{
						Name:       "my-broker",
						Namespace:  "default",
						APIVersion: "eventing.knative.dev/v1",
						Kind:       "Broker",
					},
				}: {
					self: &duckv1.Destination{
						Ref: &duckv1.KReference{
							Name:       "my-broker",
							Namespace:  "default",
							APIVersion: "eventing.knative.dev/v1",
							Kind:       "Broker",
						},
					},
				},
			},
		}, {
			name: "DLS",
			broker: eventingv1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-broker",
					Namespace: "default",
				},
				Spec: eventingv1.BrokerSpec{
					Delivery: &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							URI: sampleUri,
						},
					},
				},
			},
			expected: map[comparableDestination]*Vertex{
				{
					Ref: duckv1.KReference{
						Name:       "my-broker",
						Namespace:  "default",
						APIVersion: "eventing.knative.dev/v1",
						Kind:       "Broker",
					},
				}: brokerWithEdge,
				{
					URI: *sampleUri,
				}: destinationWithEdge,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGraph()
			g.AddBroker(test.broker)
			checkTestResult(t, g.vertices, test.expected)
		})
	}
}

func TestAddTrigger(t *testing.T) {
	tests := []struct {
		name     string
		brokers  []eventingv1.Broker
		triggers []eventingv1.Trigger
		expected map[comparableDestination]*Vertex
		wantErr  bool
	}{
		{
			name: "One Trigger, One Broker, no DLS",
			brokers: []eventingv1.Broker{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-broker",
					Namespace: "default",
				},
			}},
			triggers: []eventingv1.Trigger{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-trigger",
					Namespace: "default",
				},
				Spec: eventingv1.TriggerSpec{
					Broker: "my-broker",
					Subscriber: duckv1.Destination{
						URI: sampleUri,
					},
				},
			}},
			expected: map[comparableDestination]*Vertex{
				{
					Ref: duckv1.KReference{
						Name:       "my-broker",
						Namespace:  "default",
						APIVersion: "eventing.knative.dev/v1",
						Kind:       "Broker",
					},
				}: {
					self: &duckv1.Destination{
						Ref: &duckv1.KReference{
							Name:       "my-broker",
							Namespace:  "default",
							APIVersion: "eventing.knative.dev/v1",
							Kind:       "Broker",
						},
					},
					outEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-trigger",
									Namespace:  "default",
									APIVersion: "eventing.knative.dev/v1",
									Kind:       "Trigger",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: sampleUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-broker",
										Namespace:  "default",
										APIVersion: "eventing.knative.dev/v1",
										Kind:       "Broker",
									},
								},
							},
							transform: NoTransform{},
						},
					},
				},
				{
					URI: *sampleUri,
				}: {
					self: &duckv1.Destination{
						URI: sampleUri,
					},
					inEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-trigger",
									Namespace:  "default",
									APIVersion: "eventing.knative.dev/v1",
									Kind:       "Trigger",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: sampleUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-broker",
										Namespace:  "default",
										APIVersion: "eventing.knative.dev/v1",
										Kind:       "Broker",
									},
								},
							},
							transform: NoTransform{},
						},
					},
				},
			},
		},
		{
			name: "One Trigger, One Broker, both with DLS",
			brokers: []eventingv1.Broker{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-broker",
					Namespace: "default",
				},
				Spec: eventingv1.BrokerSpec{
					Delivery: &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							URI: secondUri,
						},
					},
				},
			}},
			triggers: []eventingv1.Trigger{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-trigger",
					Namespace: "default",
				},
				Spec: eventingv1.TriggerSpec{
					Broker: "my-broker",
					Subscriber: duckv1.Destination{
						URI: sampleUri,
					},
					Delivery: &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							URI: secondUri,
						},
					},
				},
			}},
			expected: map[comparableDestination]*Vertex{
				{
					Ref: duckv1.KReference{
						Name:       "my-broker",
						Namespace:  "default",
						APIVersion: "eventing.knative.dev/v1",
						Kind:       "Broker",
					},
				}: {
					self: &duckv1.Destination{
						Ref: &duckv1.KReference{
							Name:       "my-broker",
							Namespace:  "default",
							APIVersion: "eventing.knative.dev/v1",
							Kind:       "Broker",
						},
					},
					outEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-trigger",
									Namespace:  "default",
									APIVersion: "eventing.knative.dev/v1",
									Kind:       "Trigger",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: sampleUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-broker",
										Namespace:  "default",
										APIVersion: "eventing.knative.dev/v1",
										Kind:       "Broker",
									},
								},
							},
							transform: NoTransform{},
						},
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-trigger",
									Namespace:  "default",
									APIVersion: "eventing.knative.dev/v1",
									Kind:       "Trigger",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: secondUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-broker",
										Namespace:  "default",
										APIVersion: "eventing.knative.dev/v1",
										Kind:       "Broker",
									},
								},
							},
							transform: NoTransform{},
						},
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-broker",
									Namespace:  "default",
									APIVersion: "eventing.knative.dev/v1",
									Kind:       "Broker",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: secondUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-broker",
										Namespace:  "default",
										APIVersion: "eventing.knative.dev/v1",
										Kind:       "Broker",
									},
								},
							},
							transform: NoTransform{},
						},
					},
				},
				{
					URI: *sampleUri,
				}: {
					self: &duckv1.Destination{
						URI: sampleUri,
					},
					inEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-trigger",
									Namespace:  "default",
									APIVersion: "eventing.knative.dev/v1",
									Kind:       "Trigger",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: sampleUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-broker",
										Namespace:  "default",
										APIVersion: "eventing.knative.dev/v1",
										Kind:       "Broker",
									},
								},
							},
							transform: NoTransform{},
						},
					},
				},
				{
					URI: *secondUri,
				}: {
					self: &duckv1.Destination{
						URI: secondUri,
					},
					inEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-trigger",
									Namespace:  "default",
									APIVersion: "eventing.knative.dev/v1",
									Kind:       "Trigger",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: secondUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-broker",
										Namespace:  "default",
										APIVersion: "eventing.knative.dev/v1",
										Kind:       "Broker",
									},
								},
							},
							transform: NoTransform{},
						},
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-broker",
									Namespace:  "default",
									APIVersion: "eventing.knative.dev/v1",
									Kind:       "Broker",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: secondUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-broker",
										Namespace:  "default",
										APIVersion: "eventing.knative.dev/v1",
										Kind:       "Broker",
									},
								},
							},
							transform: NoTransform{},
						},
					},
				},
			},
		},
		{
			name:    "no broker",
			brokers: []eventingv1.Broker{},
			triggers: []eventingv1.Trigger{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-trigger",
						Namespace: "default",
					},
					Spec: eventingv1.TriggerSpec{
						Broker: "my-broker",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGraph()
			for _, broker := range test.brokers {
				g.AddBroker(broker)
			}
			for _, trigger := range test.triggers {
				err := g.AddTrigger(trigger)
				if err != nil {
					assert.True(t, test.wantErr)
					return
				}
			}
			checkTestResult(t, g.vertices, test.expected)
		})
	}
}

func TestAddSubscription(t *testing.T) {
	tests := []struct {
		name          string
		channels      []messagingv1.Channel
		subscriptions []messagingv1.Subscription
		expected      map[comparableDestination]*Vertex
		wantErr       bool
	}{
		{
			name: "One Subscriber, One Channel, no DLS, no reply",
			channels: []messagingv1.Channel{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-channel",
					Namespace: "default",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "messaging.knative.dev/v1",
					Kind:       "Channel",
				},
			}},
			subscriptions: []messagingv1.Subscription{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-subscription",
					Namespace: "default",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "messaging.knative.dev/v1",
					Kind:       "Subscription",
				},
				Spec: messagingv1.SubscriptionSpec{
					Channel: duckv1.KReference{
						Name:       "my-channel",
						Kind:       "Channel",
						APIVersion: "messaging.knative.dev/v1",
					},
					Subscriber: &duckv1.Destination{
						URI: sampleUri,
					},
				},
			}},
			expected: map[comparableDestination]*Vertex{
				{
					Ref: duckv1.KReference{
						Name:       "my-channel",
						Namespace:  "default",
						APIVersion: "messaging.knative.dev/v1",
						Kind:       "Channel",
					},
				}: {
					self: &duckv1.Destination{
						Ref: &duckv1.KReference{
							Name:       "my-channel",
							Namespace:  "default",
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Channel",
						},
					},
					outEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-subscription",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Subscription",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: sampleUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-channel",
										Namespace:  "default",
										APIVersion: "messaging.knative.dev/v1",
										Kind:       "Channel",
									},
								},
							},
							transform: NoTransform{},
						},
					},
				},
				{
					URI: *sampleUri,
				}: {
					self: &duckv1.Destination{
						URI: sampleUri,
					},
					inEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-subscription",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Subscription",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: sampleUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-channel",
										Namespace:  "default",
										APIVersion: "messaging.knative.dev/v1",
										Kind:       "Channel",
									},
								},
							},
							transform: NoTransform{},
						},
					},
				},
			},
		},
		{
			name: "One Subscription, One Channel, both with DLS, neither has reply",
			channels: []messagingv1.Channel{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-channel",
					Namespace: "default",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "messaging.knative.dev/v1",
					Kind:       "Channel",
				},
				Spec: messagingv1.ChannelSpec{ChannelableSpec: eventingduckv1.ChannelableSpec{
					Delivery: &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							URI: secondUri,
						},
					},
				},
				}}},
			subscriptions: []messagingv1.Subscription{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-subscription",
					Namespace: "default",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "messaging.knative.dev/v1",
					Kind:       "Subscription",
				},
				Spec: messagingv1.SubscriptionSpec{
					Channel: duckv1.KReference{
						Name:       "my-channel",
						Kind:       "Channel",
						APIVersion: "messaging.knative.dev/v1",
					},
					Subscriber: &duckv1.Destination{
						URI: sampleUri,
					},
					Delivery: &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							URI: secondUri,
						},
					},
				},
			}},
			expected: map[comparableDestination]*Vertex{
				{
					Ref: duckv1.KReference{
						Name:       "my-channel",
						Namespace:  "default",
						APIVersion: "messaging.knative.dev/v1",
						Kind:       "Channel",
					},
				}: {
					self: &duckv1.Destination{
						Ref: &duckv1.KReference{
							Name:       "my-channel",
							Namespace:  "default",
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Channel",
						},
					},
					outEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-subscription",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Subscription",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: sampleUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-channel",
										Namespace:  "default",
										APIVersion: "messaging.knative.dev/v1",
										Kind:       "Channel",
									},
								},
							},
							transform: NoTransform{},
						},
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-subscription",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Subscription",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: secondUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-channel",
										Namespace:  "default",
										APIVersion: "messaging.knative.dev/v1",
										Kind:       "Channel",
									},
								},
							},
							transform: NoTransform{},
						},
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-channel",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Channel",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: secondUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-channel",
										Namespace:  "default",
										APIVersion: "messaging.knative.dev/v1",
										Kind:       "Channel",
									},
								},
							},
							transform: NoTransform{},
						},
					},
				},
				{
					URI: *sampleUri,
				}: {
					self: &duckv1.Destination{
						URI: sampleUri,
					},
					inEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-subscription",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Subscription",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: sampleUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-channel",
										Namespace:  "default",
										APIVersion: "messaging.knative.dev/v1",
										Kind:       "Channel",
									},
								},
							},
							transform: NoTransform{},
						},
					},
				},
				{
					URI: *secondUri,
				}: {
					self: &duckv1.Destination{
						URI: secondUri,
					},
					inEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-subscription",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Subscription",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: secondUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-channel",
										Namespace:  "default",
										APIVersion: "messaging.knative.dev/v1",
										Kind:       "Channel",
									},
								},
							},
							transform: NoTransform{},
						},
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-channel",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Channel",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: secondUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-channel",
										Namespace:  "default",
										APIVersion: "messaging.knative.dev/v1",
										Kind:       "Channel",
									},
								},
							},
							transform: NoTransform{},
						},
					},
				},
			},
		},
		{
			name: "One Subscription, One Channel, has reply, and both has DLS",
			channels: []messagingv1.Channel{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-channel",
					Namespace: "default",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "messaging.knative.dev/v1",
					Kind:       "Channel",
				},
				Spec: messagingv1.ChannelSpec{ChannelableSpec: eventingduckv1.ChannelableSpec{
					Delivery: &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							URI: secondUri,
						},
					},
				},
				}}},
			subscriptions: []messagingv1.Subscription{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-subscription",
					Namespace: "default",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "messaging.knative.dev/v1",
					Kind:       "Subscription",
				},
				Spec: messagingv1.SubscriptionSpec{
					Channel: duckv1.KReference{
						Name:       "my-channel",
						Kind:       "Channel",
						APIVersion: "messaging.knative.dev/v1",
					},
					Subscriber: &duckv1.Destination{
						URI: sampleUri,
					},
					Delivery: &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							URI: secondUri,
						},
					},
					Reply: &duckv1.Destination{
						URI: thirdUri,
					},
				},
			}},
			expected: map[comparableDestination]*Vertex{
				{
					Ref: duckv1.KReference{
						Name:       "my-channel",
						Namespace:  "default",
						APIVersion: "messaging.knative.dev/v1",
						Kind:       "Channel",
					},
				}: {
					self: &duckv1.Destination{
						Ref: &duckv1.KReference{
							Name:       "my-channel",
							Namespace:  "default",
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Channel",
						},
					},
					outEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-subscription",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Subscription",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: sampleUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-channel",
										Namespace:  "default",
										APIVersion: "messaging.knative.dev/v1",
										Kind:       "Channel",
									},
								},
							},
							transform: NoTransform{},
						},
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-subscription",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Subscription",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: secondUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-channel",
										Namespace:  "default",
										APIVersion: "messaging.knative.dev/v1",
										Kind:       "Channel",
									},
								},
							},
							transform: NoTransform{},
						},
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-channel",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Channel",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: secondUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-channel",
										Namespace:  "default",
										APIVersion: "messaging.knative.dev/v1",
										Kind:       "Channel",
									},
								},
							},
							transform: NoTransform{},
						},
					},
				},
				{
					URI: *sampleUri,
				}: {
					self: &duckv1.Destination{
						URI: sampleUri,
					},
					inEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-subscription",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Subscription",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: sampleUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-channel",
										Namespace:  "default",
										APIVersion: "messaging.knative.dev/v1",
										Kind:       "Channel",
									},
								},
							},
							transform: NoTransform{},
						},
					},
					outEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-subscription",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Subscription",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: thirdUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									URI: sampleUri,
								},
							},
							transform: NoTransform{},
						},
					},
				},
				{
					URI: *secondUri,
				}: {
					self: &duckv1.Destination{
						URI: secondUri,
					},
					inEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-subscription",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Subscription",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: secondUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-channel",
										Namespace:  "default",
										APIVersion: "messaging.knative.dev/v1",
										Kind:       "Channel",
									},
								},
							},
							transform: NoTransform{},
						},
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-channel",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Channel",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: secondUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-channel",
										Namespace:  "default",
										APIVersion: "messaging.knative.dev/v1",
										Kind:       "Channel",
									},
								},
							},
							transform: NoTransform{},
						},
					},
				},
				{
					URI: *thirdUri,
				}: {
					self: &duckv1.Destination{
						URI: thirdUri,
					},
					inEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-subscription",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Subscription",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: thirdUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									URI: sampleUri,
								},
							},
							transform: NoTransform{},
						},
					},
				},
			},
		},
		{
			name: "One Subscription, One Channel, has reply, channel has DLS but subscription does not",
			channels: []messagingv1.Channel{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-channel",
					Namespace: "default",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "messaging.knative.dev/v1",
					Kind:       "Channel",
				},
				Spec: messagingv1.ChannelSpec{ChannelableSpec: eventingduckv1.ChannelableSpec{
					Delivery: &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							URI: secondUri,
						},
					},
				},
				}}},
			subscriptions: []messagingv1.Subscription{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-subscription",
					Namespace: "default",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "messaging.knative.dev/v1",
					Kind:       "Subscription",
				},
				Spec: messagingv1.SubscriptionSpec{
					Channel: duckv1.KReference{
						Name:       "my-channel",
						Kind:       "Channel",
						APIVersion: "messaging.knative.dev/v1",
					},
					Subscriber: &duckv1.Destination{
						URI: sampleUri,
					},
					Reply: &duckv1.Destination{
						URI: thirdUri,
					},
				},
			}},
			expected: map[comparableDestination]*Vertex{
				{
					Ref: duckv1.KReference{
						Name:       "my-channel",
						Namespace:  "default",
						APIVersion: "messaging.knative.dev/v1",
						Kind:       "Channel",
					},
				}: {
					self: &duckv1.Destination{
						Ref: &duckv1.KReference{
							Name:       "my-channel",
							Namespace:  "default",
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Channel",
						},
					},
					outEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-subscription",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Subscription",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: sampleUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-channel",
										Namespace:  "default",
										APIVersion: "messaging.knative.dev/v1",
										Kind:       "Channel",
									},
								},
							},
							transform: NoTransform{},
						},
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-channel",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Channel",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: secondUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-channel",
										Namespace:  "default",
										APIVersion: "messaging.knative.dev/v1",
										Kind:       "Channel",
									},
								},
							},
							transform: NoTransform{},
						},
					},
				},
				{
					URI: *sampleUri,
				}: {
					self: &duckv1.Destination{
						URI: sampleUri,
					},
					inEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-subscription",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Subscription",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: sampleUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-channel",
										Namespace:  "default",
										APIVersion: "messaging.knative.dev/v1",
										Kind:       "Channel",
									},
								},
							},
							transform: NoTransform{},
						},
					},
					outEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-subscription",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Subscription",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: thirdUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									URI: sampleUri,
								},
							},
							transform: NoTransform{},
						},
					},
				},
				{
					URI: *secondUri,
				}: {
					self: &duckv1.Destination{
						URI: secondUri,
					},
					inEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-channel",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Channel",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: secondUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-channel",
										Namespace:  "default",
										APIVersion: "messaging.knative.dev/v1",
										Kind:       "Channel",
									},
								},
							},
							transform: NoTransform{},
						},
					},
				},
				{
					URI: *thirdUri,
				}: {
					self: &duckv1.Destination{
						URI: thirdUri,
					},
					inEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-subscription",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Subscription",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: thirdUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									URI: sampleUri,
								},
							},
							transform: NoTransform{},
						},
					},
				},
			},
		},
		{
			name: "One Subscription, One Channel, has reply, subscription has DLS but channel does not",
			channels: []messagingv1.Channel{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-channel",
					Namespace: "default",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "messaging.knative.dev/v1",
					Kind:       "Channel",
				},
			}},
			subscriptions: []messagingv1.Subscription{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-subscription",
					Namespace: "default",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "messaging.knative.dev/v1",
					Kind:       "Subscription",
				},
				Spec: messagingv1.SubscriptionSpec{
					Channel: duckv1.KReference{
						Name:       "my-channel",
						Kind:       "Channel",
						APIVersion: "messaging.knative.dev/v1",
					},
					Subscriber: &duckv1.Destination{
						URI: sampleUri,
					},
					Delivery: &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							URI: secondUri,
						},
					},
					Reply: &duckv1.Destination{
						URI: thirdUri,
					},
				},
			}},
			expected: map[comparableDestination]*Vertex{
				{
					Ref: duckv1.KReference{
						Name:       "my-channel",
						Namespace:  "default",
						APIVersion: "messaging.knative.dev/v1",
						Kind:       "Channel",
					},
				}: {
					self: &duckv1.Destination{
						Ref: &duckv1.KReference{
							Name:       "my-channel",
							Namespace:  "default",
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Channel",
						},
					},
					outEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-subscription",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Subscription",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: sampleUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-channel",
										Namespace:  "default",
										APIVersion: "messaging.knative.dev/v1",
										Kind:       "Channel",
									},
								},
							},
							transform: NoTransform{},
						},
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-subscription",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Subscription",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: secondUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-channel",
										Namespace:  "default",
										APIVersion: "messaging.knative.dev/v1",
										Kind:       "Channel",
									},
								},
							},
							transform: NoTransform{},
						},
					},
				},
				{
					URI: *sampleUri,
				}: {
					self: &duckv1.Destination{
						URI: sampleUri,
					},
					inEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-subscription",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Subscription",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: sampleUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-channel",
										Namespace:  "default",
										APIVersion: "messaging.knative.dev/v1",
										Kind:       "Channel",
									},
								},
							},
							transform: NoTransform{},
						},
					},
					outEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-subscription",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Subscription",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: thirdUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									URI: sampleUri,
								},
							},
							transform: NoTransform{},
						},
					},
				},
				{
					URI: *secondUri,
				}: {
					self: &duckv1.Destination{
						URI: secondUri,
					},
					inEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-subscription",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Subscription",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: secondUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-channel",
										Namespace:  "default",
										APIVersion: "messaging.knative.dev/v1",
										Kind:       "Channel",
									},
								},
							},
							transform: NoTransform{},
						},
					},
				},
				{
					URI: *thirdUri,
				}: {
					self: &duckv1.Destination{
						URI: thirdUri,
					},
					inEdges: []*Edge{
						{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-subscription",
									Namespace:  "default",
									APIVersion: "messaging.knative.dev/v1",
									Kind:       "Subscription",
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									URI: thirdUri,
								},
							},
							from: &Vertex{
								self: &duckv1.Destination{
									URI: sampleUri,
								},
							},
							transform: NoTransform{},
						},
					},
				},
			},
		},
		{
			name:     "no channel",
			channels: []messagingv1.Channel{},
			subscriptions: []messagingv1.Subscription{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-subscription",
						Namespace: "default",
					},
					TypeMeta: metav1.TypeMeta{
						APIVersion: "messaging.knative.dev/v1",
						Kind:       "Subscription",
					},
					Spec: messagingv1.SubscriptionSpec{
						Channel: duckv1.KReference{
							Name:       "my-channel",
							Kind:       "Channel",
							APIVersion: "messaging.knative.dev/v1",
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGraph()
			for _, channel := range test.channels {
				g.AddChannel(channel)
			}
			for _, subscription := range test.subscriptions {
				err := g.AddSubscription(subscription)
				if err != nil {
					assert.True(t, test.wantErr)
					return
				}
			}
			checkTestResult(t, g.vertices, test.expected)
		})
	}
}

func TestAddChannel(t *testing.T) {
	channelWithEdge := &Vertex{
		self: &duckv1.Destination{
			Ref: &duckv1.KReference{
				Name:       "my-channel",
				Namespace:  "default",
				APIVersion: "messaging.knative.dev/v1",
				Kind:       "Channel",
			},
		},
	}
	destinationWithEdge := &Vertex{
		self: &duckv1.Destination{
			URI: sampleUri,
		},
	}
	channelWithEdge.AddEdge(destinationWithEdge, &duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       "my-channel",
			Namespace:  "default",
			APIVersion: "messaging.knative.dev/v1",
			Kind:       "Channel",
		},
	}, NoTransform{}, true)
	tests := []struct {
		name     string
		channel  messagingv1.Channel
		expected map[comparableDestination]*Vertex
	}{
		{
			name: "no DLS",
			channel: messagingv1.Channel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-channel",
					Namespace: "default",
				},
			},
			expected: map[comparableDestination]*Vertex{
				{
					Ref: duckv1.KReference{
						Name:       "my-channel",
						Namespace:  "default",
						APIVersion: "messaging.knative.dev/v1",
						Kind:       "Channel",
					},
				}: {
					self: &duckv1.Destination{
						Ref: &duckv1.KReference{
							Name:       "my-channel",
							Namespace:  "default",
							APIVersion: "messaging.knative.dev/v1",
							Kind:       "Channel",
						},
					},
				},
			},
		}, {
			name: "DLS",
			channel: messagingv1.Channel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-channel",
					Namespace: "default",
				},
				Spec: messagingv1.ChannelSpec{
					ChannelableSpec: eventingduckv1.ChannelableSpec{
						Delivery: &eventingduckv1.DeliverySpec{
							DeadLetterSink: &duckv1.Destination{
								URI: sampleUri,
							},
						},
					},
				},
			},
			expected: map[comparableDestination]*Vertex{
				{
					Ref: duckv1.KReference{
						Name:       "my-channel",
						Namespace:  "default",
						APIVersion: "messaging.knative.dev/v1",
						Kind:       "Channel",
					},
				}: channelWithEdge,
				{
					URI: *sampleUri,
				}: destinationWithEdge,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGraph()
			g.AddChannel(test.channel)
			checkTestResult(t, g.vertices, test.expected)
		})
	}
}

// TODO(Cali0707): add tests for event types on replies once trigger and subscriptions are merged
func TestAddEventType(t *testing.T) {
	tests := []struct {
		name     string
		et       *eventingv1beta3.EventType
		expected map[comparableDestination]*Vertex
	}{
		{
			name: "ET references source",
			et: &eventingv1beta3.EventType{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-EventType",
					Namespace: "default",
				},
				Spec: eventingv1beta3.EventTypeSpec{
					Reference: &duckv1.KReference{
						Name:       "my-source",
						Namespace:  "default",
						APIVersion: "sources.knative.dev/v1",
						Kind:       "PingSource",
					},
				},
			},
			expected: map[comparableDestination]*Vertex{
				{
					Ref: duckv1.KReference{Name: "my-EventType", Namespace: "default", APIVersion: "eventing.knative.dev/v1beta3", Kind: "EventType"},
				}: {
					self: &duckv1.Destination{
						Ref: &duckv1.KReference{Name: "my-EventType", Namespace: "default", APIVersion: "eventing.knative.dev/v1beta3", Kind: "EventType"},
					},
					outEdges: []*Edge{{
						from: &Vertex{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{Name: "my-EventType", Namespace: "default", APIVersion: "eventing.knative.dev/v1beta3", Kind: "EventType"},
							},
						},
						to: &Vertex{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{Name: "my-source", Namespace: "default", APIVersion: "sources.knative.dev/v1", Kind: "PingSource"},
							},
						},
						self: &duckv1.Destination{
							Ref: &duckv1.KReference{Name: "my-EventType", Namespace: "default", APIVersion: "eventing.knative.dev/v1beta3", Kind: "EventType"},
						},
						transform: EventTypeTransform{},
					}},
				},
				{
					Ref: duckv1.KReference{Name: "my-source", Namespace: "default", APIVersion: "sources.knative.dev/v1", Kind: "PingSource"},
				}: {
					self: &duckv1.Destination{
						Ref: &duckv1.KReference{Name: "my-source", Namespace: "default", APIVersion: "sources.knative.dev/v1", Kind: "PingSource"},
					},
					inEdges: []*Edge{{
						from: &Vertex{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{Name: "my-EventType", Namespace: "default", APIVersion: "eventing.knative.dev/v1beta3", Kind: "EventType"},
							},
						},
						to: &Vertex{
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{Name: "my-source", Namespace: "default", APIVersion: "sources.knative.dev/v1", Kind: "PingSource"},
							},
						},
						self: &duckv1.Destination{
							Ref: &duckv1.KReference{Name: "my-EventType", Namespace: "default", APIVersion: "eventing.knative.dev/v1beta3", Kind: "EventType"},
						},
						transform: EventTypeTransform{},
					}},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGraph()
			g.AddEventType(test.et)
			checkTestResult(t, g.vertices, test.expected)
		})

	}
}

func TestAddSource(t *testing.T) {
	tests := []struct {
		name     string
		source   duckv1.Source
		expected map[comparableDestination]*Vertex
	}{
		{
			name: "no CE Overrides",
			source: duckv1.Source{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-source",
					Namespace: "default",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "sources.knative.dev/v1",
					Kind:       "PingSource",
				},
				Spec: duckv1.SourceSpec{
					Sink: duckv1.Destination{
						Ref: &duckv1.KReference{
							Name:       "my-broker",
							Namespace:  "default",
							APIVersion: "eventing.knative.dev/v1",
							Kind:       "Broker",
						},
					},
				},
			},
			expected: map[comparableDestination]*Vertex{
				{
					Ref: duckv1.KReference{
						Name:       "my-broker",
						Namespace:  "default",
						APIVersion: "eventing.knative.dev/v1",
						Kind:       "Broker",
					},
				}: {
					self: &duckv1.Destination{
						Ref: &duckv1.KReference{
							Name:       "my-broker",
							Namespace:  "default",
							APIVersion: "eventing.knative.dev/v1",
							Kind:       "Broker",
						},
					},
					inEdges: []*Edge{
						{
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-source",
										Namespace:  "default",
										APIVersion: "sources.knative.dev/v1",
										Kind:       "PingSource",
									},
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-broker",
										Namespace:  "default",
										APIVersion: "eventing.knative.dev/v1",
										Kind:       "Broker",
									},
								},
							},
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-source",
									Namespace:  "default",
									APIVersion: "sources.knative.dev/v1",
									Kind:       "PingSource",
								},
							},
							transform: CloudEventOverridesTransform{},
						},
					},
				},
				{
					Ref: duckv1.KReference{
						Name:       "my-source",
						Namespace:  "default",
						APIVersion: "sources.knative.dev/v1",
						Kind:       "PingSource",
					},
				}: {
					self: &duckv1.Destination{
						Ref: &duckv1.KReference{
							Name:       "my-source",
							Namespace:  "default",
							APIVersion: "sources.knative.dev/v1",
							Kind:       "PingSource",
						},
					},
					outEdges: []*Edge{
						{
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-source",
										Namespace:  "default",
										APIVersion: "sources.knative.dev/v1",
										Kind:       "PingSource",
									},
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-broker",
										Namespace:  "default",
										APIVersion: "eventing.knative.dev/v1",
										Kind:       "Broker",
									},
								},
							},
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-source",
									Namespace:  "default",
									APIVersion: "sources.knative.dev/v1",
									Kind:       "PingSource",
								},
							},
							transform: CloudEventOverridesTransform{},
						},
					},
				},
			},
		},
		{
			name: "CE Overrides",
			source: duckv1.Source{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-source",
					Namespace: "default",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: "sources.knative.dev/v1",
					Kind:       "PingSource",
				},
				Spec: duckv1.SourceSpec{
					Sink: duckv1.Destination{
						Ref: &duckv1.KReference{
							Name:       "my-broker",
							Namespace:  "default",
							APIVersion: "eventing.knative.dev/v1",
							Kind:       "Broker",
						},
					},
					CloudEventOverrides: &duckv1.CloudEventOverrides{Extensions: map[string]string{"someextension": "somevalue"}},
				},
			},
			expected: map[comparableDestination]*Vertex{
				{
					Ref: duckv1.KReference{
						Name:       "my-broker",
						Namespace:  "default",
						APIVersion: "eventing.knative.dev/v1",
						Kind:       "Broker",
					},
				}: {
					self: &duckv1.Destination{
						Ref: &duckv1.KReference{
							Name:       "my-broker",
							Namespace:  "default",
							APIVersion: "eventing.knative.dev/v1",
							Kind:       "Broker",
						},
					},
					inEdges: []*Edge{
						{
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-source",
										Namespace:  "default",
										APIVersion: "sources.knative.dev/v1",
										Kind:       "PingSource",
									},
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-broker",
										Namespace:  "default",
										APIVersion: "eventing.knative.dev/v1",
										Kind:       "Broker",
									},
								},
							},
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-source",
									Namespace:  "default",
									APIVersion: "sources.knative.dev/v1",
									Kind:       "PingSource",
								},
							},
							transform: CloudEventOverridesTransform{Overrides: &duckv1.CloudEventOverrides{Extensions: map[string]string{"someextension": "somevalue"}}},
						},
					},
				},
				{
					Ref: duckv1.KReference{
						Name:       "my-source",
						Namespace:  "default",
						APIVersion: "sources.knative.dev/v1",
						Kind:       "PingSource",
					},
				}: {
					self: &duckv1.Destination{
						Ref: &duckv1.KReference{
							Name:       "my-source",
							Namespace:  "default",
							APIVersion: "sources.knative.dev/v1",
							Kind:       "PingSource",
						},
					},
					outEdges: []*Edge{
						{
							from: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-source",
										Namespace:  "default",
										APIVersion: "sources.knative.dev/v1",
										Kind:       "PingSource",
									},
								},
							},
							to: &Vertex{
								self: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Name:       "my-broker",
										Namespace:  "default",
										APIVersion: "eventing.knative.dev/v1",
										Kind:       "Broker",
									},
								},
							},
							self: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Name:       "my-source",
									Namespace:  "default",
									APIVersion: "sources.knative.dev/v1",
									Kind:       "PingSource",
								},
							},
							transform: CloudEventOverridesTransform{Overrides: &duckv1.CloudEventOverrides{Extensions: map[string]string{"someextension": "somevalue"}}},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGraph()
			g.AddSource(test.source)
			checkTestResult(t, g.vertices, test.expected)
		})
	}
}

func checkTestResult(t *testing.T, actualVertices map[comparableDestination]*Vertex, expectedVertices map[comparableDestination]*Vertex) {
	assert.Len(t, actualVertices, len(expectedVertices))
	for k, expected := range expectedVertices {
		actual, ok := actualVertices[k]
		assert.True(t, ok)
		// assert.Equal can't do equality on function values, which edges have, so we need to do a more complicated check
		assert.Equal(t, actual.self, expected.self)
		assert.Len(t, actual.inEdges, len(expected.inEdges))
		assert.Subset(t, makeComparableEdges(actual.inEdges), makeComparableEdges(expected.inEdges))
		assert.Len(t, actual.outEdges, len(expected.outEdges))
		assert.Subset(t, makeComparableEdges(actual.outEdges), makeComparableEdges(expected.outEdges))
	}
}

func makeComparableEdges(edges []*Edge) []comparableEdge {
	if len(edges) == 0 {
		return []comparableEdge{}
	}

	res := make([]comparableEdge, 0, len(edges))

	for _, e := range edges {
		res = append(res, comparableEdge{
			self:          makeComparableDestination(e.self),
			to:            makeComparableDestination(e.to.self),
			from:          makeComparableDestination(e.from.self),
			transformName: e.transform.Name(),
		})
	}

	return res
}

type comparableEdge struct {
	self          comparableDestination
	to            comparableDestination
	from          comparableDestination
	transformName string
}
