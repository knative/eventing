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

package v1alpha1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
)

func TestChoiceValidation(t *testing.T) {
	name := "invalid choice spec"
	choice := &Choice{Spec: ChoiceSpec{}}

	want := &apis.FieldError{
		Paths:   []string{"spec.channelTemplate", "spec.cases"},
		Message: "missing field(s)",
	}

	t.Run(name, func(t *testing.T) {
		got := choice.Validate(context.TODO())
		if diff := cmp.Diff(want.Error(), got.Error()); diff != "" {
			t.Errorf("Choice.Validate (-want, +got) = %v", diff)
		}
	})
}

func TestChoiceSpecValidation(t *testing.T) {
	subscriberURI := "http://example.com"
	validChannelTemplate := &eventingduck.ChannelTemplateSpec{
		metav1.TypeMeta{
			Kind:       "mykind",
			APIVersion: "myapiversion",
		},
		&runtime.RawExtension{},
	}
	tests := []struct {
		name string
		ts   *ChoiceSpec
		want *apis.FieldError
	}{{
		name: "invalid choice spec - empty",
		ts:   &ChoiceSpec{},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channelTemplate", "cases")
			return fe
		}(),
	}, {
		name: "invalid choice spec - empty cases",
		ts: &ChoiceSpec{
			ChannelTemplate: validChannelTemplate,
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("cases")
			return fe
		}(),
	}, {
		name: "missing channeltemplatespec",
		ts: &ChoiceSpec{
			Cases: []ChoiceCase{{Subscriber: eventingv1alpha1.SubscriberSpec{URI: &subscriberURI}}},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channelTemplate")
			return fe
		}(),
	}, {
		name: "invalid channeltemplatespec missing APIVersion",
		ts: &ChoiceSpec{
			ChannelTemplate: &eventingduck.ChannelTemplateSpec{metav1.TypeMeta{Kind: "mykind"}, &runtime.RawExtension{}},
			Cases:           []ChoiceCase{{Subscriber: eventingv1alpha1.SubscriberSpec{URI: &subscriberURI}}},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channelTemplate.apiVersion")
			return fe
		}(),
	}, {
		name: "invalid channeltemplatespec missing Kind",
		ts: &ChoiceSpec{
			ChannelTemplate: &eventingduck.ChannelTemplateSpec{metav1.TypeMeta{APIVersion: "myapiversion"}, &runtime.RawExtension{}},
			Cases:           []ChoiceCase{{Subscriber: eventingv1alpha1.SubscriberSpec{URI: &subscriberURI}}},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channelTemplate.kind")
			return fe
		}(),
	}, {
		name: "valid choice",
		ts: &ChoiceSpec{
			ChannelTemplate: validChannelTemplate,
			Cases:           []ChoiceCase{{Subscriber: eventingv1alpha1.SubscriberSpec{URI: &subscriberURI}}},
		},
		want: func() *apis.FieldError {
			return nil
		}(),
	}, {
		name: "valid choice with valid reply",
		ts: &ChoiceSpec{
			ChannelTemplate: validChannelTemplate,
			Cases:           []ChoiceCase{{Subscriber: eventingv1alpha1.SubscriberSpec{URI: &subscriberURI}}},
			Reply:           makeValidReply("reply-channel"),
		},
		want: func() *apis.FieldError {
			return nil
		}(),
	}, {
		name: "valid choice with invalid missing name",
		ts: &ChoiceSpec{
			ChannelTemplate: validChannelTemplate,
			Cases:           []ChoiceCase{{Subscriber: eventingv1alpha1.SubscriberSpec{URI: &subscriberURI}}},
			Reply: &corev1.ObjectReference{
				APIVersion: "messaging.knative.dev/v1alpha1",
				Kind:       "inmemorychannel",
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("reply.name")
			return fe
		}(),
	}, {
		name: "valid choice with invalid reply",
		ts: &ChoiceSpec{
			ChannelTemplate: validChannelTemplate,
			Cases:           []ChoiceCase{{Subscriber: eventingv1alpha1.SubscriberSpec{URI: &subscriberURI}}},
			Reply:           makeInvalidReply("reply-channel"),
		},
		want: func() *apis.FieldError {
			fe := apis.ErrDisallowedFields("reply.Namespace")
			fe.Details = "only name, apiVersion and kind are supported fields"
			return fe
		}(),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.ts.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: Validate ChoiceSpec (-want, +got) = %v", test.name, diff)
			}
		})
	}
}
