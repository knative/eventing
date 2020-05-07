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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-cmp/cmp"
	"knative.dev/eventing/pkg/apis/messaging/config"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestParallelSetDefaults(t *testing.T) {
	testCases := map[string]struct {
		nilChannelDefaulter bool
		channelTemplate     *config.ChannelTemplateSpec
		initial             Parallel
		expected            Parallel
	}{
		"nil ChannelDefaulter": {
			nilChannelDefaulter: true,
			expected:            Parallel{},
		},
		"unset ChannelDefaulter": {
			expected: Parallel{},
		},
		"set ChannelDefaulter": {
			channelTemplate: configDefaultChannelTemplate,
			expected: Parallel{
				Spec: ParallelSpec{
					ChannelTemplate: defaultChannelTemplate,
				},
			},
		},
		"branches namespace defaulted": {
			channelTemplate: configDefaultChannelTemplate,
			initial: Parallel{
				ObjectMeta: metav1.ObjectMeta{Namespace: testNS},
				Spec: ParallelSpec{
					Branches: []ParallelBranch{
						{
							Filter: &duckv1.Destination{
								Ref: &duckv1.KReference{Name: "firstfilter"},
							},
							Subscriber: duckv1.Destination{
								Ref: &duckv1.KReference{Name: "firstsub"},
							},
							Reply: &duckv1.Destination{
								Ref: &duckv1.KReference{Name: "firstreply"},
							},
						}, {
							Filter: &duckv1.Destination{
								Ref: &duckv1.KReference{Name: "secondfilter"},
							},
							Subscriber: duckv1.Destination{
								Ref: &duckv1.KReference{Name: "secondsub"},
							},
							Reply: &duckv1.Destination{
								Ref: &duckv1.KReference{Name: "secondreply"},
							},
						}},
					Reply: &duckv1.Destination{Ref: &duckv1.KReference{Name: "reply"}}},
			},
			expected: Parallel{
				ObjectMeta: metav1.ObjectMeta{Namespace: testNS},
				Spec: ParallelSpec{
					ChannelTemplate: defaultChannelTemplate,
					Branches: []ParallelBranch{
						{
							Filter: &duckv1.Destination{
								Ref: &duckv1.KReference{Name: "firstfilter", Namespace: testNS},
							},
							Subscriber: duckv1.Destination{
								Ref: &duckv1.KReference{Name: "firstsub", Namespace: testNS},
							},
							Reply: &duckv1.Destination{
								Ref: &duckv1.KReference{Name: "firstreply", Namespace: testNS},
							},
						}, {
							Filter: &duckv1.Destination{
								Ref: &duckv1.KReference{Name: "secondfilter", Namespace: testNS},
							},
							Subscriber: duckv1.Destination{
								Ref: &duckv1.KReference{Name: "secondsub", Namespace: testNS},
							},
							Reply: &duckv1.Destination{
								Ref: &duckv1.KReference{Name: "secondreply", Namespace: testNS},
							},
						}},
					Reply: &duckv1.Destination{Ref: &duckv1.KReference{Name: "reply", Namespace: testNS}},
				},
			},
		},
		"template already specified": {
			channelTemplate: configDefaultChannelTemplate,
			initial: Parallel{
				Spec: ParallelSpec{
					ChannelTemplate: &messagingv1beta1.ChannelTemplateSpec{
						TypeMeta: v1.TypeMeta{
							APIVersion: SchemeGroupVersion.String(),
							Kind:       "OtherChannel",
						},
					},
				},
			},
			expected: Parallel{
				Spec: ParallelSpec{
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
				t.Fatalf("Unexpected defaults (-want, +got): %s", diff)
			}
		})
	}
}
