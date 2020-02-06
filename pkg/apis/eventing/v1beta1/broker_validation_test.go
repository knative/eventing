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
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestBrokerImmutableFields(t *testing.T) {
	original := &Broker{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"eventing.knative.dev/broker.class": "original"},
		},
	}
	current := &Broker{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"eventing.knative.dev/broker.class": "current"},
		},
	}

	tests := map[string]struct {
		og      *Broker
		wantErr *apis.FieldError
	}{
		"nil original": {
			wantErr: nil,
		},
		"no ChannelTemplateSpec mutation": {
			og:      current,
			wantErr: nil,
		},
		"ChannelTemplateSpec mutated": {
			og: original,
			wantErr: &apis.FieldError{
				Message: "Immutable fields changed (-old +new)",
				Paths:   []string{"annotations"},
				Details: `{string}:
	-: "original"
	+: "current"
`,
			},
		},
	}

	for n, test := range tests {
		t.Run(n, func(t *testing.T) {
			gotErr := current.CheckImmutableFields(context.Background(), test.og)
			if diff := cmp.Diff(test.wantErr.Error(), gotErr.Error()); diff != "" {
				t.Errorf("Broker.CheckImmutableFields (-want, +got) = %v", diff)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	invalidString := "invalid time"
	tests := []struct {
		name string
		b    Broker
		want *apis.FieldError
	}{{
		name: "valid empty",
		b:    Broker{},
	}, {
		name: "valid config",
		b: Broker{
			Spec: BrokerSpec{
				Config: &duckv1.KReference{
					Namespace:  "namespace",
					Name:       "name",
					Kind:       "kind",
					APIVersion: "apiversion",
				},
			},
		},
	}, {
		name: "valid config, no namespace",
		b: Broker{
			Spec: BrokerSpec{
				Config: &duckv1.KReference{
					Name:       "name",
					Kind:       "kind",
					APIVersion: "apiversion",
				},
			},
		},
	}, {
		name: "invalid config, missing name",
		b: Broker{
			Spec: BrokerSpec{
				Config: &duckv1.KReference{
					Namespace:  "namespace",
					Kind:       "kind",
					APIVersion: "apiversion",
				},
			},
		},
		want: apis.ErrMissingField("spec.config.name"),
	}, {
		name: "invalid config, missing apiVersion",
		b: Broker{
			Spec: BrokerSpec{
				Config: &duckv1.KReference{
					Namespace: "namespace",
					Name:      "name",
					Kind:      "kind",
				},
			},
		},
		want: apis.ErrMissingField("spec.config.apiVersion"),
	}, {
		name: "invalid config, missing kind",
		b: Broker{
			Spec: BrokerSpec{
				Config: &duckv1.KReference{
					Namespace:  "namespace",
					Name:       "name",
					APIVersion: "apiversion",
				},
			},
		},
		want: apis.ErrMissingField("spec.config.kind"),
	}, {
		name: "invalid delivery, invalid delay string",
		b: Broker{
			Spec: BrokerSpec{
				Delivery: &eventingduckv1beta1.DeliverySpec{
					BackoffDelay: &invalidString,
				},
			},
		},
		want: apis.ErrInvalidValue(invalidString, "spec.delivery.backoffDelay"),
	}, {}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.b.Validate(context.Background())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("BrokerSpec.Validate (-want, +got) = %v", diff)
			}
		})
	}
}

func TestValidSpec(t *testing.T) {
	bop := eventingduckv1beta1.BackoffPolicyExponential
	tests := []struct {
		name string
		spec BrokerSpec
		want *apis.FieldError
	}{{
		name: "valid empty",
		spec: BrokerSpec{},
	}, {
		name: "valid config",
		spec: BrokerSpec{
			Config: &duckv1.KReference{
				Namespace:  "namespace",
				Name:       "name",
				Kind:       "kind",
				APIVersion: "apiversion",
			},
		},
	}, {
		name: "valid delivery",
		spec: BrokerSpec{
			Config: &duckv1.KReference{
				Namespace:  "namespace",
				Name:       "name",
				Kind:       "kind",
				APIVersion: "apiversion",
			},
			Delivery: &eventingduckv1beta1.DeliverySpec{BackoffPolicy: &bop},
		},
	}, {}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.spec.Validate(context.Background())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("BrokerSpec.Validate (-want, +got) = %v", diff)
			}
		})
	}
}
