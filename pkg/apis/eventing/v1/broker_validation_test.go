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

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
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
		"no BrokerClassAnnotation mutation": {
			og:      current,
			wantErr: nil,
		},
		"BrokerClassAnnotation mutated": {
			og: original,
			wantErr: &apis.FieldError{
				Message: "Immutable annotations changed (-old +new)",
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
			ctx := context.Background()
			ctx = apis.WithinUpdate(ctx, test.og)
			got := current.Validate(ctx)
			if diff := cmp.Diff(test.wantErr.Error(), got.Error()); diff != "" {
				t.Error("Broker.CheckImmutableFields (-want, +got) =", diff)
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
		name: "missing annotation",
		b:    Broker{},
		want: apis.ErrMissingField("eventing.knative.dev/broker.class"),
	}, {
		name: "empty annotation",
		b: Broker{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"eventing.knative.dev/broker.class": ""},
			},
		},
		want: apis.ErrMissingField("eventing.knative.dev/broker.class"),
	}, {
		name: "valid empty",
		b: Broker{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"eventing.knative.dev/broker.class": "MTChannelBasedBroker"},
			},
		},
	}, {
		name: "valid config",
		b: Broker{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"eventing.knative.dev/broker.class": "MTChannelBasedBroker"},
			},
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
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"eventing.knative.dev/broker.class": "MTChannelBasedBroker"},
			},
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
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"eventing.knative.dev/broker.class": "MTChannelBasedBroker"},
			},
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
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"eventing.knative.dev/broker.class": "MTChannelBasedBroker"},
			},
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
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"eventing.knative.dev/broker.class": "MTChannelBasedBroker"},
			},
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
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"eventing.knative.dev/broker.class": "MTChannelBasedBroker"},
			},
			Spec: BrokerSpec{
				Delivery: &eventingduckv1.DeliverySpec{
					BackoffDelay: &invalidString,
				},
			},
		},
		want: apis.ErrInvalidValue(invalidString, "spec.delivery.backoffDelay"),
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.b.Validate(context.Background())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Error("Broker.Validate (-want, +got) =", diff)
			}
		})
	}
}

func TestValidateUpdate(t *testing.T) {
	tests := []struct {
		name string
		b    Broker
		bNew Broker
		want *apis.FieldError
	}{{
		name: "invalid config change, spec.config",
		b: Broker{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"eventing.knative.dev/broker.class": "MTChannelBasedBroker"},
			},
			Spec: BrokerSpec{
				Config: &duckv1.KReference{
					Namespace:  "namespace",
					Name:       "name",
					Kind:       "kind",
					APIVersion: "apiversion",
				},
			},
		},
		bNew: Broker{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"eventing.knative.dev/broker.class": "MTChannelBasedBroker"},
			},
			Spec: BrokerSpec{
				Config: &duckv1.KReference{
					Namespace:  "namespace",
					Name:       "name2",
					Kind:       "kind",
					APIVersion: "apiversion",
				},
			},
		},
		want: &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: `{v1.BrokerSpec}.Config.Name:
	-: "name"
	+: "name2"
`,
		},
	}, {
		name: "invalid config change, broker.class",
		b: Broker{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"eventing.knative.dev/broker.class": "MTChannelBasedBroker"},
			},
			Spec: BrokerSpec{
				Config: &duckv1.KReference{
					Namespace:  "namespace",
					Name:       "name",
					Kind:       "kind",
					APIVersion: "apiversion",
				},
			},
		},
		bNew: Broker{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"eventing.knative.dev/broker.class": "SomeOtherBrokerClass"},
			},
			Spec: BrokerSpec{
				Config: &duckv1.KReference{
					Namespace:  "namespace",
					Name:       "name",
					Kind:       "kind",
					APIVersion: "apiversion",
				},
			},
		},
		want: &apis.FieldError{
			Message: "Immutable annotations changed (-old +new)",
			Paths:   []string{"annotations"},
			Details: `{string}:
	-: "MTChannelBasedBroker"
	+: "SomeOtherBrokerClass"
`,
		},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := apis.WithinUpdate(context.Background(), &test.b)
			got := test.bNew.Validate(ctx)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Error("Broker.Validate (-want, +got) =", diff)
			}
		})
	}
}

func TestValidSpec(t *testing.T) {
	bop := eventingduckv1.BackoffPolicyExponential
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
			Delivery: &eventingduckv1.DeliverySpec{BackoffPolicy: &bop},
		},
	}, {}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.spec.Validate(context.Background())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Error("BrokerSpec.Validate (-want, +got) =", diff)
			}
		})
	}
}
