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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-cmp/cmp"
	"knative.dev/eventing/pkg/apis/messaging/config"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

const (
	testNS = "testnamespace"
)

var (
	configDefaultChannelTemplate = &config.ChannelTemplateSpec{
		TypeMeta: v1.TypeMeta{
			APIVersion: SchemeGroupVersion.String(),
			Kind:       "InMemoryChannel",
		},
	}
)

func TestSequenceSetDefaults(t *testing.T) {
	testCases := map[string]struct {
		nilChannelDefaulter bool
		channelTemplate     *config.ChannelTemplateSpec
		initial             Sequence
		expected            Sequence
	}{
		"nil ChannelDefaulter": {
			nilChannelDefaulter: true,
			expected:            Sequence{},
		},
		"unset ChannelDefaulter": {
			expected: Sequence{},
		},
		"set ChannelDefaulter": {
			channelTemplate: configDefaultChannelTemplate,
			expected: Sequence{
				Spec: SequenceSpec{
					ChannelTemplate: defaultChannelTemplate,
				},
			},
		},
		"steps and reply namespace defaulted": {
			channelTemplate: configDefaultChannelTemplate,
			initial: Sequence{
				ObjectMeta: metav1.ObjectMeta{Namespace: testNS},
				Spec: SequenceSpec{
					Steps: []SequenceStep{
						{Destination: duckv1.Destination{
							Ref: &duckv1.KReference{Name: "first"}}},
						{Destination: duckv1.Destination{
							Ref: &duckv1.KReference{Name: "second"}}},
					},
					Reply: &duckv1.Destination{
						Ref: &duckv1.KReference{Name: "reply"},
					},
				},
			},
			expected: Sequence{
				ObjectMeta: metav1.ObjectMeta{Namespace: testNS},
				Spec: SequenceSpec{
					ChannelTemplate: defaultChannelTemplate,
					Steps: []SequenceStep{
						{Destination: duckv1.Destination{
							Ref: &duckv1.KReference{Namespace: testNS, Name: "first"}}},
						{Destination: duckv1.Destination{
							Ref: &duckv1.KReference{Namespace: testNS, Name: "second"}}},
					},
					Reply: &duckv1.Destination{
						Ref: &duckv1.KReference{Namespace: testNS, Name: "reply"},
					},
				},
			},
		},
		"template already specified": {
			channelTemplate: configDefaultChannelTemplate,
			initial: Sequence{
				Spec: SequenceSpec{
					ChannelTemplate: &messagingv1beta1.ChannelTemplateSpec{
						TypeMeta: v1.TypeMeta{
							APIVersion: SchemeGroupVersion.String(),
							Kind:       "OtherChannel",
						},
					},
				},
			},
			expected: Sequence{
				Spec: SequenceSpec{
					ChannelTemplate: &messagingv1beta1.ChannelTemplateSpec{
						TypeMeta: v1.TypeMeta{
							APIVersion: SchemeGroupVersion.String(),
							Kind:       "OtherChannel",
						},
					},
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			ctx := context.Background()
			if !tc.nilChannelDefaulter {
				ctx = config.ToContext(ctx, &config.Config{
					ChannelDefaults: &config.ChannelDefaults{
						ClusterDefault: tc.channelTemplate,
					},
				})
			}
			tc.initial.SetDefaults(ctx)
			if diff := cmp.Diff(tc.expected, tc.initial); diff != "" {
				t.Fatal("Unexpected defaults (-want, +got):", diff)
			}
		})
	}
}
