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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func getValidSteps() []SequenceStep {
	return []SequenceStep{
		{
			Destination: getValidDestination(),
			Delivery:    getValidDelivery(),
		},
	}
}

func getInvalidSteps() []SequenceStep {
	return []SequenceStep{
		{
			Destination: getValidDestination(),
			Delivery:    getInvalidDelivery(),
		},
	}
}

func getValidChannelTemplate() *messagingv1beta1.ChannelTemplateSpec {
	return &messagingv1beta1.ChannelTemplateSpec{
		TypeMeta: v1.TypeMeta{
			APIVersion: "testAPIVersion",
			Kind:       "testChannel",
		},
	}
}

func getValidDestination() duckv1.Destination {
	return duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       "testName",
			Kind:       "testKind",
			Namespace:  "testNamespace",
			APIVersion: "testAPIVersion",
		},
	}
}

func getValidDestinationRef() *duckv1.Destination {
	d := getValidDestination()
	return &d
}

func getInvalidDestination() duckv1.Destination {
	return duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:      "testName",
			Kind:      "testKind",
			Namespace: "testNamespace",
		},
	}
}

func getInvalidDestinationRef() *duckv1.Destination {
	d := getInvalidDestination()
	return &d
}

func getValidDelivery() *eventingduckv1beta1.DeliverySpec {
	bop := eventingduckv1beta1.BackoffPolicyExponential
	return &eventingduckv1beta1.DeliverySpec{
		BackoffPolicy: &bop,
	}
}

func getInvalidDelivery() *eventingduckv1beta1.DeliverySpec {
	invalidBod := "invalid delay"
	return &eventingduckv1beta1.DeliverySpec{
		BackoffDelay: &invalidBod,
	}
}

func TestSequenceValidate(t *testing.T) {
	tests := []struct {
		name string
		s    *Sequence
		want *apis.FieldError
	}{
		{
			name: "valid",
			s: &Sequence{
				Spec: SequenceSpec{
					Steps:           getValidSteps(),
					ChannelTemplate: getValidChannelTemplate(),
					Reply:           getValidDestinationRef(),
				},
			},
			want: nil,
		},
		{
			name: "invalid",
			s: &Sequence{
				Spec: SequenceSpec{
					ChannelTemplate: getValidChannelTemplate(),
					Reply:           getValidDestinationRef(),
				},
			},
			want: apis.ErrMissingField("spec.steps"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.s.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: Sequence.Validate (-want, +got) = %v", test.name, diff)
			}
		})
	}
}

func TestSequenceSpecValidate(t *testing.T) {
	invalidSteps := getInvalidSteps()

	tests := []struct {
		name string
		ss   *SequenceSpec
		want *apis.FieldError
	}{
		{
			name: "valid",
			ss: &SequenceSpec{
				Steps:           getValidSteps(),
				ChannelTemplate: getValidChannelTemplate(),
				Reply:           getValidDestinationRef(),
			},
			want: nil,
		},
		{
			name: "no steps",
			ss: &SequenceSpec{
				ChannelTemplate: getValidChannelTemplate(),
				Reply:           getValidDestinationRef(),
			},
			want: apis.ErrMissingField("steps"),
		},
		{
			name: "invalid steps",
			ss: &SequenceSpec{
				Steps:           invalidSteps,
				ChannelTemplate: getValidChannelTemplate(),
				Reply:           getValidDestinationRef(),
			},
			want: apis.ErrInvalidArrayValue(invalidSteps[0], "steps", 0),
		},
		{
			name: "no channelTemplate",
			ss: &SequenceSpec{
				Steps: getValidSteps(),
				Reply: getValidDestinationRef(),
			},
			want: apis.ErrMissingField("channelTemplate"),
		},
		{
			name: "no channelTemplate apiVersion",
			ss: &SequenceSpec{
				Steps: getValidSteps(),
				ChannelTemplate: &messagingv1beta1.ChannelTemplateSpec{
					TypeMeta: v1.TypeMeta{
						Kind: "testChannel",
					},
				},
				Reply: getValidDestinationRef(),
			},
			want: apis.ErrMissingField("channelTemplate.apiVersion"),
		},
		{
			name: "no channelTemplate kind",
			ss: &SequenceSpec{
				Steps: getValidSteps(),
				ChannelTemplate: &messagingv1beta1.ChannelTemplateSpec{
					TypeMeta: v1.TypeMeta{
						APIVersion: "testAPIVersion",
					},
				},
				Reply: getValidDestinationRef(),
			},
			want: apis.ErrMissingField("channelTemplate.kind"),
		},
		{
			name: "invalid channelTemplate & invalid reply",
			ss: &SequenceSpec{
				Steps: getValidSteps(),
				ChannelTemplate: &messagingv1beta1.ChannelTemplateSpec{
					TypeMeta: v1.TypeMeta{
						Kind: "testChannel",
					},
				},
				Reply: getInvalidDestinationRef(),
			},
			want: apis.ErrMissingField("channelTemplate.apiVersion", "reply.ref.apiVersion"),
		},
		{
			name: "invalid reply",
			ss: &SequenceSpec{
				Steps:           getValidSteps(),
				ChannelTemplate: getValidChannelTemplate(),
				Reply:           getInvalidDestinationRef(),
			},
			want: apis.ErrMissingField("reply.ref.apiVersion"),
		},
		{
			name: "no channel template & invalid reply",
			ss: &SequenceSpec{
				Steps: getValidSteps(),
				Reply: getInvalidDestinationRef(),
			},
			want: apis.ErrMissingField("channelTemplate", "reply.ref.apiVersion"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.ss.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: SequenceSpec.Validate (-want, +got) = %v", test.name, diff)
			}
		})
	}
}

func TestSequenceStepValidate(t *testing.T) {
	tests := []struct {
		name string
		ss   *SequenceStep
		want *apis.FieldError
	}{
		{
			name: "valid",
			ss: &SequenceStep{
				Destination: getValidDestination(),
				Delivery:    getValidDelivery(),
			},
			want: nil,
		},
		{
			name: "invalid destination",
			ss: &SequenceStep{
				Destination: getInvalidDestination(),
				Delivery:    getValidDelivery(),
			},
			want: apis.ErrMissingField("ref.apiVersion"),
		},
		{
			name: "invalid delivery",
			ss: &SequenceStep{
				Destination: getValidDestination(),
				Delivery:    getInvalidDelivery(),
			},
			want: apis.ErrInvalidValue("invalid delay", "delivery.backoffDelay"),
		},
		{
			name: "invalid destination & invalid delivery",
			ss: &SequenceStep{
				Destination: getInvalidDestination(),
				Delivery:    getInvalidDelivery(),
			},
			want: func() *apis.FieldError {
				errs := apis.ErrMissingField("ref.apiVersion")
				return errs.Also(apis.ErrInvalidValue("invalid delay", "delivery.backoffDelay"))
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.ss.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: SequenceStep.Validate (-want, +got) = %v", test.name, diff)
			}
		})
	}
}
