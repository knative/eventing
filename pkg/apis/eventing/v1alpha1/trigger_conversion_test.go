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

package v1alpha1

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// TODO: Replace dummy some other Eventing object once they
// implement apis.Convertible
type dummy struct{}

func (*dummy) ConvertUp(ctx context.Context, obj apis.Convertible) error {
	return errors.New("Won't go")
}

func (*dummy) ConvertDown(ctx context.Context, obj apis.Convertible) error {
	return errors.New("Won't go")
}

func TestTriggerConversionBadType(t *testing.T) {
	good, bad := &Trigger{}, &dummy{}

	if err := good.ConvertUp(context.Background(), bad); err == nil {
		t.Errorf("ConvertUp() = %#v, wanted error", bad)
	}

	if err := good.ConvertDown(context.Background(), bad); err == nil {
		t.Errorf("ConvertDown() = %#v, wanted error", good)
	}
}

func TestTriggerConversion(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&v1beta1.Trigger{}}

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
	}, {name: "filter rules, deprecated",
		in: &Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "trigger-name",
				Namespace:  "trigger-ns",
				Generation: 17,
			},
			Spec: TriggerSpec{
				Broker: "default",
				Filter: &TriggerFilter{
					DeprecatedSourceAndType: &TriggerFilterSourceAndType{
						Source: "mysource",
						Type:   "mytype",
					},
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
					Attributes: &TriggerFilterAttributes{"source": "mysource", "type": "mytype"},
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
					Attributes: &TriggerFilterAttributes{"source": "mysource", "type": "mytype", "customkey": "customvalue"},
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
	}}
	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := test.in.ConvertUp(context.Background(), ver); err != nil {
					t.Errorf("ConvertUp() = %v", err)
				}
				got := &Trigger{}
				if err := got.ConvertDown(context.Background(), ver); err != nil {
					t.Errorf("ConvertDown() = %v", err)
				}
				// Since on the way down, we lose the DeprecatedSourceAndType,
				// convert the in to equivalent out.
				fixed := fixDeprecated(test.in)
				if diff := cmp.Diff(fixed, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}

// Since DeprecatedSourceAndType is lossy but semanctically equivalent
// if source,type are present and equivalent in the attributes map,
// fix that so diff works.
func fixDeprecated(in *Trigger) *Trigger {
	if in.Spec.Filter != nil && in.Spec.Filter.DeprecatedSourceAndType != nil {
		// attributes must be nil, can't have both Deprecated / Attributes
		attributes := TriggerFilterAttributes{}
		attributes["source"] = in.Spec.Filter.DeprecatedSourceAndType.Source
		attributes["type"] = in.Spec.Filter.DeprecatedSourceAndType.Type
		in.Spec.Filter.DeprecatedSourceAndType = nil
		in.Spec.Filter.Attributes = &attributes
	}
	return in
}
