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
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/pkg/apis"
	//	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestPipelineValidation(t *testing.T) {
	name := "invalid pipeline spec"
	pipeline := &Pipeline{Spec: PipelineSpec{}}

	want := &apis.FieldError{
		Paths:   []string{"spec.channelTemplate", "spec.steps"},
		Message: "missing field(s)",
	}

	t.Run(name, func(t *testing.T) {
		got := pipeline.Validate(context.TODO())
		if diff := cmp.Diff(want.Error(), got.Error()); diff != "" {
			t.Errorf("Pipeline.Validate (-want, +got) = %v", diff)
		}
	})
}

func TestPipelineSpecValidation(t *testing.T) {
	subscriberURI := "http://example.com"
	validChannelTemplate := ChannelTemplateSpec{
		metav1.TypeMeta{
			Kind:       "mykind",
			APIVersion: "myapiversion",
		},
		runtime.RawExtension{},
	}
	tests := []struct {
		name string
		ts   *PipelineSpec
		want *apis.FieldError
	}{{
		name: "invalid pipeline spec - empty",
		ts:   &PipelineSpec{},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channelTemplate", "steps")
			return fe
		}(),
	}, {
		name: "invalid pipeline spec - empty steps",
		ts: &PipelineSpec{
			ChannelTemplate: validChannelTemplate,
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("steps")
			return fe
		}(),
	}, {
		name: "missing channeltemplatespec",
		ts: &PipelineSpec{
			Steps: []eventingv1alpha1.SubscriberSpec{{URI: &subscriberURI}},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channelTemplate")
			return fe
		}(),
	}, {
		name: "invalid channeltemplatespec missing APIVersion",
		ts: &PipelineSpec{
			ChannelTemplate: ChannelTemplateSpec{metav1.TypeMeta{Kind: "mykind"}, runtime.RawExtension{}},
			Steps:           []eventingv1alpha1.SubscriberSpec{{URI: &subscriberURI}},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channelTemplate.apiVersion")
			return fe
		}(),
	}, {
		name: "invalid channeltemplatespec missing Kind",
		ts: &PipelineSpec{
			ChannelTemplate: ChannelTemplateSpec{metav1.TypeMeta{APIVersion: "myapiversion"}, runtime.RawExtension{}},
			Steps:           []eventingv1alpha1.SubscriberSpec{{URI: &subscriberURI}},
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("channelTemplate.kind")
			return fe
		}(),
	}, {
		name: "valid pipeline",
		ts: &PipelineSpec{
			ChannelTemplate: validChannelTemplate,
			Steps:           []eventingv1alpha1.SubscriberSpec{{URI: &subscriberURI}},
		},
		want: func() *apis.FieldError {
			return nil
		}(),
		/*
			}, {
				name: "missing filter.sourceAndType",
				ts: &PipelineSpec{
					Broker:     "test_broker",
					Filter:     &PipelineFilter{},
					Subscriber: validSubscriber,
				},
				want: func() *apis.FieldError {
					fe := apis.ErrMissingField("filter.sourceAndType")
					return fe
				}(),
			}, {
				name: "missing subscriber",
				ts: &PipelineSpec{
					Broker: "test_broker",
					Filter: validPipelineFilter,
				},
				want: func() *apis.FieldError {
					fe := apis.ErrMissingField("subscriber")
					return fe
				}(),
			}, {
				name: "missing subscriber.ref.name",
				ts: &PipelineSpec{
					Broker:     "test_broker",
					Filter:     validPipelineFilter,
					Subscriber: invalidSubscriber,
				},
				want: func() *apis.FieldError {
					fe := apis.ErrMissingField("subscriber.ref.name")
					return fe
				}(),
		*/
	},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.ts.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: Validate PipelineSpec (-want, +got) = %v", test.name, diff)
			}
		})
	}
}
