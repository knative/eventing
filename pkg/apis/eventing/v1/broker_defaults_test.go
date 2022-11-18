/*
Copyright 2020 The Knative Authors

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

package v1

import (
	"context"
	"testing"

	"k8s.io/utils/pointer"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	duckv1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/eventing/pkg/apis/config"
	"knative.dev/eventing/pkg/apis/eventing"
)

var (
	defaultConfig = &config.Config{
		Defaults: &config.Defaults{
			// NamespaceDefaultsConfig are the default Broker Configs for each namespace.
			// Namespace is the key, the value is the KReference to the config.
			NamespaceDefaultsConfig: map[string]*config.ClassAndBrokerConfig{
				"mynamespace": {
					BrokerConfig: &config.BrokerConfig{
						KReference: &duckv1.KReference{
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Namespace:  "knative-eventing",
							Name:       "kafka-channel",
						},
						Delivery: &eventingduckv1.DeliverySpec{
							DeadLetterSink: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Kind:       "Service",
									Namespace:  "default",
									Name:       "handle-error",
									APIVersion: "serving.knative.dev/v1",
								},
							},
							Retry:         pointer.Int32(5),
							BackoffPolicy: (*eventingduckv1.BackoffPolicyType)(pointer.String("exponential")),
							BackoffDelay:  pointer.String("5s"),
						},
					},
				},
				"mynamespace2": {
					BrokerClass: "mynamespace2class",
					BrokerConfig: &config.BrokerConfig{
						KReference: &duckv1.KReference{
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Namespace:  "knative-eventing",
							Name:       "natss-channel",
						},
						Delivery: &eventingduckv1.DeliverySpec{
							DeadLetterSink: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Kind:       "Service",
									Namespace:  "knative-eventing",
									Name:       "handle-error",
									APIVersion: "serving.knative.dev/v1",
								},
							},
							Retry:         pointer.Int32(3),
							BackoffPolicy: (*eventingduckv1.BackoffPolicyType)(pointer.String("exponential")),
							BackoffDelay:  pointer.String("3s"),
						},
					},
				},
				"mynamespace3": {
					BrokerClass: "mynamespace3class",
					BrokerConfig: &config.BrokerConfig{
						KReference: &duckv1.KReference{
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Name:       "natss-channel",
						},
						Delivery: &eventingduckv1.DeliverySpec{
							DeadLetterSink: &duckv1.Destination{
								Ref: &duckv1.KReference{
									Kind:       "Service",
									Namespace:  "mynamespace3",
									Name:       "handle-error",
									APIVersion: "serving.knative.dev/v1",
								},
							},
							Retry:         pointer.Int32(5),
							BackoffPolicy: (*eventingduckv1.BackoffPolicyType)(pointer.String("linear")),
							BackoffDelay:  pointer.String("5s"),
						},
					},
				},
			},
			ClusterDefault: &config.ClassAndBrokerConfig{
				BrokerClass: eventing.MTChannelBrokerClassValue,
				BrokerConfig: &config.BrokerConfig{
					KReference: &duckv1.KReference{
						APIVersion: "v1",
						Kind:       "ConfigMap",
						Namespace:  "knative-eventing",
						Name:       "imc-channel",
					},
					Delivery: &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "Service",
								Namespace:  "knative-eventing",
								Name:       "handle-error",
								APIVersion: "serving.knative.dev/v1",
							},
						},
						Retry:         pointer.Int32(3),
						BackoffPolicy: (*eventingduckv1.BackoffPolicyType)(pointer.String("exponential")),
						BackoffDelay:  pointer.String("5s"),
					},
				},
			},
		},
	}
)

func TestBrokerSetDefaults(t *testing.T) {
	testCases := map[string]struct {
		initial  Broker
		expected Broker
	}{
		"default everything from cluster": {
			expected: Broker{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						eventing.BrokerClassKey: eventing.MTChannelBrokerClassValue,
					},
				},
				Spec: BrokerSpec{
					Config: &duckv1.KReference{
						APIVersion: "v1",
						Kind:       "ConfigMap",
						Namespace:  "knative-eventing",
						Name:       "imc-channel",
					},
					Delivery: &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "Service",
								Namespace:  "knative-eventing",
								Name:       "handle-error",
								APIVersion: "serving.knative.dev/v1",
							},
						},
						Retry:         pointer.Int32(3),
						BackoffPolicy: (*eventingduckv1.BackoffPolicyType)(pointer.String("exponential")),
						BackoffDelay:  pointer.String("5s"),
					},
				},
			},
		},
		"default annotation from cluster": {
			expected: Broker{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						eventing.BrokerClassKey: eventing.MTChannelBrokerClassValue,
					},
				},
				Spec: BrokerSpec{
					Config: &duckv1.KReference{
						APIVersion: "v1",
						Kind:       "ConfigMap",
						Namespace:  "knative-eventing",
						Name:       "imc-channel",
					},
					Delivery: &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "Service",
								Namespace:  "knative-eventing",
								Name:       "handle-error",
								APIVersion: "serving.knative.dev/v1",
							},
						},
						Retry:         pointer.Int32(3),
						BackoffPolicy: (*eventingduckv1.BackoffPolicyType)(pointer.String("exponential")),
						BackoffDelay:  pointer.String("5s"),
					},
				},
			},
			initial: Broker{
				Spec: BrokerSpec{
					Config: &duckv1.KReference{
						APIVersion: "v1",
						Kind:       "ConfigMap",
						Namespace:  "knative-eventing",
						Name:       "imc-channel",
					},
				},
			},
		},
		"config already specified, adds annotation": {
			initial: Broker{
				Spec: BrokerSpec{
					Config: &duckv1.KReference{
						APIVersion: SchemeGroupVersion.String(),
						Kind:       "OtherChannel",
					},
				},
			},
			expected: Broker{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						eventing.BrokerClassKey: eventing.MTChannelBrokerClassValue,
					},
				},
				Spec: BrokerSpec{
					Config: &duckv1.KReference{
						APIVersion: SchemeGroupVersion.String(),
						Kind:       "OtherChannel",
					},
					Delivery: &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "Service",
								Namespace:  "knative-eventing",
								Name:       "handle-error",
								APIVersion: "serving.knative.dev/v1",
							},
						},
						Retry:         pointer.Int32(3),
						BackoffPolicy: (*eventingduckv1.BackoffPolicyType)(pointer.String("exponential")),
						BackoffDelay:  pointer.String("5s"),
					},
				},
			},
		},
		"no config, uses namespace broker config, cluster class": {
			initial: Broker{
				ObjectMeta: metav1.ObjectMeta{Namespace: "mynamespace"},
			},
			expected: Broker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "mynamespace",
					Annotations: map[string]string{
						eventing.BrokerClassKey: eventing.MTChannelBrokerClassValue,
					},
				},
				Spec: BrokerSpec{
					Config: &duckv1.KReference{
						Kind:       "ConfigMap",
						Namespace:  "knative-eventing",
						Name:       "kafka-channel",
						APIVersion: "v1",
					},
					Delivery: &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "Service",
								Namespace:  "default",
								Name:       "handle-error",
								APIVersion: "serving.knative.dev/v1",
							},
						},
						Retry:         pointer.Int32(5),
						BackoffPolicy: (*eventingduckv1.BackoffPolicyType)(pointer.String("exponential")),
						BackoffDelay:  pointer.String("5s"),
					},
				},
			},
		},
		"no config, uses namespace broker config, defaults namespace, cluster class": {
			initial: Broker{
				ObjectMeta: metav1.ObjectMeta{Namespace: "mynamespace3"},
			},
			expected: Broker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "mynamespace3",
					Annotations: map[string]string{
						eventing.BrokerClassKey: "mynamespace3class",
					},
				},
				Spec: BrokerSpec{
					Config: &duckv1.KReference{
						Kind:       "ConfigMap",
						Namespace:  "mynamespace3",
						Name:       "natss-channel",
						APIVersion: "v1",
					},
					Delivery: &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "Service",
								Namespace:  "mynamespace3",
								Name:       "handle-error",
								APIVersion: "serving.knative.dev/v1",
							},
						},
						Retry:         pointer.Int32(5),
						BackoffPolicy: (*eventingduckv1.BackoffPolicyType)(pointer.String("linear")),
						BackoffDelay:  pointer.String("5s"),
					},
				},
			},
		},
		"no config, uses namespace broker config and class": {
			initial: Broker{
				ObjectMeta: metav1.ObjectMeta{Namespace: "mynamespace2"},
			},
			expected: Broker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "mynamespace2",
					Annotations: map[string]string{
						eventing.BrokerClassKey: "mynamespace2class",
					},
				},
				Spec: BrokerSpec{
					Config: &duckv1.KReference{
						Kind:       "ConfigMap",
						Namespace:  "knative-eventing",
						Name:       "natss-channel",
						APIVersion: "v1",
					},
					Delivery: &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "Service",
								Namespace:  "knative-eventing",
								Name:       "handle-error",
								APIVersion: "serving.knative.dev/v1",
							},
						},
						Retry:         pointer.Int32(3),
						BackoffPolicy: (*eventingduckv1.BackoffPolicyType)(pointer.String("exponential")),
						BackoffDelay:  pointer.String("3s"),
					},
				},
			},
		},
		"config, missing namespace, defaulted": {
			initial: Broker{
				ObjectMeta: metav1.ObjectMeta{Name: "rando", Namespace: "randons"},
				Spec: BrokerSpec{
					Config: &duckv1.KReference{
						Kind:       "ConfigMap",
						Name:       "natss-channel",
						APIVersion: "v1",
					},
					Delivery: &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "Service",
								Namespace:  "rando",
								Name:       "handle-error",
								APIVersion: "serving.knative.dev/v1",
							},
						},
						Retry:         pointer.Int32(5),
						BackoffPolicy: (*eventingduckv1.BackoffPolicyType)(pointer.String("linear")),
						BackoffDelay:  pointer.String("5s"),
					},
				},
			},
			expected: Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rando",
					Namespace: "randons",
					Annotations: map[string]string{
						eventing.BrokerClassKey: "MTChannelBasedBroker",
					},
				},
				Spec: BrokerSpec{
					Config: &duckv1.KReference{
						Kind:       "ConfigMap",
						Namespace:  "randons",
						Name:       "natss-channel",
						APIVersion: "v1",
					},
					Delivery: &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "Service",
								Namespace:  "rando",
								Name:       "handle-error",
								APIVersion: "serving.knative.dev/v1",
							},
						},
						Retry:         pointer.Int32(5),
						BackoffPolicy: (*eventingduckv1.BackoffPolicyType)(pointer.String("linear")),
						BackoffDelay:  pointer.String("5s"),
					},
				},
			},
		},
		"missing deadLetterSink.ref.namespace, defaulted": {
			initial: Broker{
				ObjectMeta: metav1.ObjectMeta{Name: "broker", Namespace: "custom"},
				Spec: BrokerSpec{
					Config: &duckv1.KReference{
						Kind:       "ConfigMap",
						Name:       "natss-channel",
						Namespace:  "custom1",
						APIVersion: "v1",
					},
					Delivery: &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "Service",
								Name:       "handle-error",
								APIVersion: "serving.knative.dev/v1",
							},
						},
						Retry:         pointer.Int32(5),
						BackoffPolicy: (*eventingduckv1.BackoffPolicyType)(pointer.String("linear")),
						BackoffDelay:  pointer.String("5s"),
					},
				},
			},
			expected: Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "broker",
					Namespace: "custom",
					Annotations: map[string]string{
						eventing.BrokerClassKey: "MTChannelBasedBroker",
					},
				},
				Spec: BrokerSpec{
					Config: &duckv1.KReference{
						Kind:       "ConfigMap",
						Namespace:  "custom1",
						Name:       "natss-channel",
						APIVersion: "v1",
					},
					Delivery: &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "Service",
								Namespace:  "custom",
								Name:       "handle-error",
								APIVersion: "serving.knative.dev/v1",
							},
						},
						Retry:         pointer.Int32(5),
						BackoffPolicy: (*eventingduckv1.BackoffPolicyType)(pointer.String("linear")),
						BackoffDelay:  pointer.String("5s"),
					},
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			tc.initial.SetDefaults(config.ToContext(context.Background(), defaultConfig))
			if diff := cmp.Diff(tc.expected, tc.initial); diff != "" {
				t.Fatal("Unexpected defaults (-want, +got):", diff)
			}
		})
	}
}
