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

package v1beta1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestTriggerConversionBadType(t *testing.T) {
	good, bad := &Trigger{}, &Trigger{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

func TestTriggerConversionBadVersion(t *testing.T) {
	good, bad := &Trigger{}, &Trigger{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

// Test v1beta1 -> v1 -> v1beta1
func TestTriggerConversionRoundTripV1beta1(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&v1.Trigger{}}

	tests := []struct {
		name string
		in   *Trigger
	}{{name: "simple configuration",
		in: &Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "trigger-name",
				Namespace:  "trigger-ns",
				Generation: 17,
			},
			Spec: TriggerSpec{
				Broker: "default",
			},
			Status: TriggerStatus{
				Status: duckv1.Status{
					ObservedGeneration: 1,
					Conditions: duckv1.Conditions{{
						Type:   "Ready",
						Status: "True",
					}},
				},
			},
		},
	}, {name: "filter rules",
		in: &Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "trigger-name",
				Namespace:  "trigger-ns",
				Generation: 17,
			},
			Spec: TriggerSpec{
				Broker: "default",
				Filter: &TriggerFilter{
					Attributes: TriggerFilterAttributes{"source": "mysource", "type": "mytype"},
				},
			},
			Status: TriggerStatus{
				Status: duckv1.Status{
					ObservedGeneration: 1,
					Conditions: duckv1.Conditions{{
						Type:   "Ready",
						Status: "True",
					}},
				},
			},
		},
	}, {name: "filter rules, many",
		in: &Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "trigger-name",
				Namespace:  "trigger-ns",
				Generation: 17,
			},
			Spec: TriggerSpec{
				Broker: "default",
				Filter: &TriggerFilter{
					Attributes: TriggerFilterAttributes{"source": "mysource", "type": "mytype", "customkey": "customvalue"},
				},
			},
			Status: TriggerStatus{
				Status: duckv1.Status{
					ObservedGeneration: 1,
					Conditions: duckv1.Conditions{{
						Type:   "Ready",
						Status: "True",
					}},
				},
			},
		},
	}, {name: "full configuration",
		in: &Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "trigger-name",
				Namespace:  "trigger-ns",
				Generation: 17,
			},
			Spec: TriggerSpec{
				Broker: "default",
				Filter: &TriggerFilter{
					Attributes: TriggerFilterAttributes{"source": "mysource", "type": "mytype"},
				},
				Subscriber: duckv1.Destination{
					Ref: &duckv1.KReference{
						Kind:       "subscriberKind",
						Namespace:  "subscriberNamespace",
						Name:       "subscriberName",
						APIVersion: "subscriberAPIVersion",
					},
					URI: apis.HTTP("subscriberURI"),
				},
			},
			Status: TriggerStatus{
				Status: duckv1.Status{
					ObservedGeneration: 1,
					Conditions: duckv1.Conditions{{
						Type:   "Ready",
						Status: "True",
					}},
				},
				SubscriberURI: apis.HTTP("subscriberURI"),
			},
		},
	}}

	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}
				got := &Trigger{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}

				if diff := cmp.Diff(test.in, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}

// Test v1 -> v1beta1 -> v1
func TestTriggerConversionRoundTripV1(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&Trigger{}}

	tests := []struct {
		name string
		in   *v1.Trigger
	}{{name: "simple configuration",
		in: &v1.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "trigger-name",
				Namespace:  "trigger-ns",
				Generation: 17,
			},
			Spec: v1.TriggerSpec{
				Broker: "default",
			},
			Status: v1.TriggerStatus{
				Status: duckv1.Status{
					ObservedGeneration: 1,
					Conditions: duckv1.Conditions{{
						Type:   "Ready",
						Status: "True",
					}},
				},
			},
		},
	}, {name: "filter rules",
		in: &v1.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "trigger-name",
				Namespace:  "trigger-ns",
				Generation: 17,
			},
			Spec: v1.TriggerSpec{
				Broker: "default",
				Filter: &v1.TriggerFilter{
					Attributes: v1.TriggerFilterAttributes{"source": "mysource", "type": "mytype"},
				},
			},
			Status: v1.TriggerStatus{
				Status: duckv1.Status{
					ObservedGeneration: 1,
					Conditions: duckv1.Conditions{{
						Type:   "Ready",
						Status: "True",
					}},
				},
			},
		},
	}, {name: "filter rules, many",
		in: &v1.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "trigger-name",
				Namespace:  "trigger-ns",
				Generation: 17,
			},
			Spec: v1.TriggerSpec{
				Broker: "default",
				Filter: &v1.TriggerFilter{
					Attributes: v1.TriggerFilterAttributes{"source": "mysource", "type": "mytype", "customkey": "customvalue"},
				},
			},
			Status: v1.TriggerStatus{
				Status: duckv1.Status{
					ObservedGeneration: 1,
					Conditions: duckv1.Conditions{{
						Type:   "Ready",
						Status: "True",
					}},
				},
			},
		},
	}, {name: "full configuration",
		in: &v1.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "trigger-name",
				Namespace:  "trigger-ns",
				Generation: 17,
			},
			Spec: v1.TriggerSpec{
				Broker: "default",
				Filter: &v1.TriggerFilter{
					Attributes: v1.TriggerFilterAttributes{"source": "mysource", "type": "mytype"},
				},
				Subscriber: duckv1.Destination{
					Ref: &duckv1.KReference{
						Kind:       "subscriberKind",
						Namespace:  "subscriberNamespace",
						Name:       "subscriberName",
						APIVersion: "subscriberAPIVersion",
					},
					URI: apis.HTTP("subscriberURI"),
				},
			},
			Status: v1.TriggerStatus{
				Status: duckv1.Status{
					ObservedGeneration: 1,
					Conditions: duckv1.Conditions{{
						Type:   "Ready",
						Status: "True",
					}},
				},
				SubscriberURI: apis.HTTP("subscriberURI"),
			},
		},
	}}

	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := ver.ConvertFrom(context.Background(), test.in); err != nil {
					t.Errorf("ConvertDown() = %v", err)
				}
				got := &v1.Trigger{}
				if err := ver.ConvertTo(context.Background(), got); err != nil {
					t.Errorf("ConvertUp() = %v", err)
				}

				if diff := cmp.Diff(test.in, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}
