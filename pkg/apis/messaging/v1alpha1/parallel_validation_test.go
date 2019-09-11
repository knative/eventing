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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/apis"
)

func TestParallelValidation(t *testing.T) {
	name := "invalid parallel spec"
	parallel := &Parallel{Spec: ParallelSpec{}}

	want := &apis.FieldError{
		Paths:   []string{"spec.channelTemplate", "spec.branches"},
		Message: "missing field(s)",
	}

	t.Run(name, func(t *testing.T) {
		got := parallel.Validate(context.TODO())
		if diff := cmp.Diff(want.Error(), got.Error()); diff != "" {
			t.Errorf("Parallel.Validate (-want, +got) = %v", diff)
		}
	})
}

func TestParallelSpecValidation(t *testing.T) {
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
		ts   *ParallelSpec
		want *apis.FieldError
	}{{
		name: "invalid parallel spec - empty",
		ts:   &ParallelSpec{},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channelTemplate", "branches")
			return fe
		}(),
	}, {
		name: "invalid parallel spec - empty branches",
		ts: &ParallelSpec{
			ChannelTemplate: validChannelTemplate,
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("branches")
			return fe
		}(),
	}, {
		name: "missing channeltemplatespec",
		ts: &ParallelSpec{
			Branches: []ParallelBranch{{Subscriber: SubscriberSpec{URI: &subscriberURI}}},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channelTemplate")
			return fe
		}(),
	}, {
		name: "invalid channeltemplatespec missing APIVersion",
		ts: &ParallelSpec{
			ChannelTemplate: &eventingduck.ChannelTemplateSpec{metav1.TypeMeta{Kind: "mykind"}, &runtime.RawExtension{}},
			Branches:        []ParallelBranch{{Subscriber: SubscriberSpec{URI: &subscriberURI}}},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channelTemplate.apiVersion")
			return fe
		}(),
	}, {
		name: "invalid channeltemplatespec missing Kind",
		ts: &ParallelSpec{
			ChannelTemplate: &eventingduck.ChannelTemplateSpec{metav1.TypeMeta{APIVersion: "myapiversion"}, &runtime.RawExtension{}},
			Branches:        []ParallelBranch{{Subscriber: SubscriberSpec{URI: &subscriberURI}}},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channelTemplate.kind")
			return fe
		}(),
	}, {
		name: "valid parallel",
		ts: &ParallelSpec{
			ChannelTemplate: validChannelTemplate,
			Branches:        []ParallelBranch{{Subscriber: SubscriberSpec{URI: &subscriberURI}}},
		},
		want: func() *apis.FieldError {
			return nil
		}(),
	}, {
		name: "valid parallel with valid reply",
		ts: &ParallelSpec{
			ChannelTemplate: validChannelTemplate,
			Branches:        []ParallelBranch{{Subscriber: SubscriberSpec{URI: &subscriberURI}}},
			Reply:           makeValidReply("reply-channel"),
		},
		want: func() *apis.FieldError {
			return nil
		}(),
	}, {
		name: "valid parallel with invalid missing name",
		ts: &ParallelSpec{
			ChannelTemplate: validChannelTemplate,
			Branches:        []ParallelBranch{{Subscriber: SubscriberSpec{URI: &subscriberURI}}},
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
		name: "valid parallel with invalid reply",
		ts: &ParallelSpec{
			ChannelTemplate: validChannelTemplate,
			Branches:        []ParallelBranch{{Subscriber: SubscriberSpec{URI: &subscriberURI}}},
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
				t.Errorf("%s: Validate ParallelSpec (-want, +got) = %v", test.name, diff)
			}
		})
	}
}
