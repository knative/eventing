/*
Copyright 2021 The Knative Authors

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
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	"knative.dev/pkg/apis"
)

func getValidBranches() []ParallelBranch {
	return []ParallelBranch{
		{
			Filter:     getValidDestinationRef(),
			Subscriber: getValidDestination(),
			Reply:      getValidDestinationRef(),
			Delivery:   getValidDelivery(),
		},
	}
}

func TestParallelValidate(t *testing.T) {
	tests := []struct {
		name string
		p    *Parallel
		want *apis.FieldError
	}{
		{
			name: "valid",
			p: &Parallel{
				Spec: ParallelSpec{
					Branches:        getValidBranches(),
					ChannelTemplate: getValidChannelTemplate(),
					Reply:           getValidDestinationRef(),
				},
			},
			want: nil,
		},
		{
			name: "invalid",
			p: &Parallel{
				Spec: ParallelSpec{
					ChannelTemplate: getValidChannelTemplate(),
					Reply:           getValidDestinationRef(),
				},
			},
			want: apis.ErrMissingField("spec.branches"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.p.Validate(context.TODO())
			if diff := cmp.Diff(tt.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: Parallel.Validate (-want, +got) = %v", tt.name, diff)
			}
		})
	}
}

func TestParallelSpecValidate(t *testing.T) {
	invalidBranches := [][]ParallelBranch{
		{{
			Filter:     getInvalidDestinationRef(),
			Subscriber: getValidDestination(),
			Reply:      getValidDestinationRef(),
			Delivery:   getValidDelivery(),
		}},
		{{
			Filter:     getValidDestinationRef(),
			Subscriber: getInvalidDestination(),
			Reply:      getValidDestinationRef(),
			Delivery:   getValidDelivery(),
		}},
		{{
			Filter:     getValidDestinationRef(),
			Subscriber: getValidDestination(),
			Reply:      getInvalidDestinationRef(),
			Delivery:   getValidDelivery(),
		}},
	}

	invalidChannelTemplates := []*messagingv1beta1.ChannelTemplateSpec{
		{
			TypeMeta: metav1.TypeMeta{
				Kind: "testChannel",
			},
		},
		{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "testAPIVersion",
			},
		},
	}

	tests := []struct {
		name string
		ps   *ParallelSpec
		want *apis.FieldError
	}{
		{
			name: "valid",
			ps: &ParallelSpec{
				Branches:        getValidBranches(),
				ChannelTemplate: getValidChannelTemplate(),
				Reply:           getValidDestinationRef(),
			},
			want: nil,
		},
		{
			name: "without branches",
			ps: &ParallelSpec{
				ChannelTemplate: getValidChannelTemplate(),
				Reply:           getValidDestinationRef(),
			},
			want: apis.ErrMissingField("branches"),
		},
		{
			name: "branches with invalid filter",
			ps: &ParallelSpec{
				Branches:        invalidBranches[0],
				ChannelTemplate: getValidChannelTemplate(),
				Reply:           getValidDestinationRef(),
			},
			want: apis.ErrInvalidArrayValue(invalidBranches[0][0], "branches.filter", 0),
		},
		{
			name: "branches with invalid subscriber",
			ps: &ParallelSpec{
				Branches:        invalidBranches[1],
				ChannelTemplate: getValidChannelTemplate(),
				Reply:           getValidDestinationRef(),
			},
			want: apis.ErrInvalidArrayValue(invalidBranches[1][0], "branches.subscriber", 0),
		},
		{
			name: "branches with invalid reply",
			ps: &ParallelSpec{
				Branches:        invalidBranches[2],
				ChannelTemplate: getValidChannelTemplate(),
				Reply:           getValidDestinationRef(),
			},
			want: apis.ErrInvalidArrayValue(invalidBranches[2][0], "branches.reply", 0),
		},
		{
			name: "without channelTemplate",
			ps: &ParallelSpec{
				Branches: getValidBranches(),
				Reply:    getValidDestinationRef(),
			},
			want: apis.ErrMissingField("channelTemplate"),
		},
		{
			name: "channelTemplate without apiVersion",
			ps: &ParallelSpec{
				Branches:        getValidBranches(),
				ChannelTemplate: invalidChannelTemplates[0],
				Reply:           getValidDestinationRef(),
			},
			want: apis.ErrMissingField("channelTemplate.apiVersion"),
		},
		{
			name: "channelTemplate without kind",
			ps: &ParallelSpec{
				Branches:        getValidBranches(),
				ChannelTemplate: invalidChannelTemplates[1],
				Reply:           getValidDestinationRef(),
			},
			want: apis.ErrMissingField("channelTemplate.kind"),
		},
		{
			name: "invalid reply",
			ps: &ParallelSpec{
				Branches:        getValidBranches(),
				ChannelTemplate: getValidChannelTemplate(),
				Reply:           getInvalidDestinationRef(),
			},
			want: apis.ErrMissingField("reply.ref.apiVersion"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ps.Validate(context.TODO())
			if diff := cmp.Diff(tt.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: ParallelSpec.Validate (-want, +got) = %v", tt.name, diff)
			}
		})
	}
}
