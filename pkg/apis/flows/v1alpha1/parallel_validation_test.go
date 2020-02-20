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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
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
	subscriberURI := apis.HTTP("example.com")
	validChannelTemplate := &messagingv1beta1.ChannelTemplateSpec{
		TypeMeta: metav1.TypeMeta{
			Kind:       "mykind",
			APIVersion: "myapiversion",
		},
		Spec: &runtime.RawExtension{},
	}
	invalidReplyInParallel := ParallelBranch{Subscriber: duckv1.Destination{URI: subscriberURI},
		Reply: makeInvalidReply("reply-channel")}

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
			Branches: []ParallelBranch{{Subscriber: duckv1.Destination{URI: subscriberURI}}},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channelTemplate")
			return fe
		}(),
	}, {
		name: "invalid channeltemplatespec missing APIVersion",
		ts: &ParallelSpec{
			ChannelTemplate: &messagingv1beta1.ChannelTemplateSpec{
				TypeMeta: metav1.TypeMeta{Kind: "mykind"},
				Spec:     &runtime.RawExtension{}},
			Branches: []ParallelBranch{{Subscriber: duckv1.Destination{URI: subscriberURI}}},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channelTemplate.apiVersion")
			return fe
		}(),
	}, {
		name: "invalid channeltemplatespec missing Kind",
		ts: &ParallelSpec{
			ChannelTemplate: &messagingv1beta1.ChannelTemplateSpec{
				TypeMeta: metav1.TypeMeta{APIVersion: "myapiversion"},
				Spec:     &runtime.RawExtension{}},
			Branches: []ParallelBranch{{Subscriber: duckv1.Destination{URI: subscriberURI}}},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channelTemplate.kind")
			return fe
		}(),
	}, {
		name: "valid parallel",
		ts: &ParallelSpec{
			ChannelTemplate: validChannelTemplate,
			Branches:        []ParallelBranch{{Subscriber: duckv1.Destination{URI: subscriberURI}}},
		},
		want: func() *apis.FieldError {
			return nil
		}(),
	}, {
		name: "valid parallel with valid reply",
		ts: &ParallelSpec{
			ChannelTemplate: validChannelTemplate,
			Branches:        []ParallelBranch{{Subscriber: duckv1.Destination{URI: subscriberURI}}},
			Reply:           makeValidReply("reply-channel"),
		},
		want: func() *apis.FieldError {
			return nil
		}(),
	}, {
		name: "parallel with invalid missing name",
		ts: &ParallelSpec{
			ChannelTemplate: validChannelTemplate,
			Branches:        []ParallelBranch{{Subscriber: duckv1.Destination{URI: subscriberURI}}},
			Reply: &duckv1.Destination{
				Ref: &duckv1.KReference{
					Namespace:  "namespace",
					APIVersion: "messaging.knative.dev/v1alpha1",
					Kind:       "inmemorychannel",
				},
			},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("reply.ref.name")
			return fe
		}(),
	}, {
		name: "parallel with invalid reply",
		ts: &ParallelSpec{
			ChannelTemplate: validChannelTemplate,
			Branches:        []ParallelBranch{{Subscriber: duckv1.Destination{URI: subscriberURI}}},
			Reply:           makeInvalidReply("reply-channel"),
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("reply.ref.apiVersion")
			return fe
		}(),
	}, {
		name: "parallel with invalid branch reply",
		ts: &ParallelSpec{
			ChannelTemplate: validChannelTemplate,
			Branches:        []ParallelBranch{invalidReplyInParallel},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrInvalidArrayValue(invalidReplyInParallel, "branches.reply", 0)
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
