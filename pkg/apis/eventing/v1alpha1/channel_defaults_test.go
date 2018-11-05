/*
Copyright 2018 The Knative Authors

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
	"testing"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"

	"github.com/google/go-cmp/cmp"
)

var (
	defaultChannelProvisioner = "default-channel-provisioner"
)

func TestChannelSetDefaults(t *testing.T) {
	testCases := map[string]struct {
		nilChannelDefaulter bool
		def                 *string
		initial             Channel
		expected            Channel
	}{
		"nil ChannelDefaulter": {
			nilChannelDefaulter: true,
			expected:            Channel{},
		},
		"unset ChannelDefaulter": {
			expected: Channel{},
		},
		"set ChannelDefaulter": {
			def: &defaultChannelProvisioner,
			expected: Channel{
				Spec: ChannelSpec{
					Provisioner: &corev1.ObjectReference{
						APIVersion: SchemeGroupVersion.String(),
						Kind:       "ClusterChannelProvisioner",
						Name:       defaultChannelProvisioner,
					},
				},
			},
		},
		"provisioner already specified": {
			def: &defaultChannelProvisioner,
			initial: Channel{
				Spec: ChannelSpec{
					Provisioner: &corev1.ObjectReference{
						APIVersion: SchemeGroupVersion.String(),
						Kind:       "ClusterChannelProvisioner",
						Name:       "already-specified",
					},
				},
			},
			expected: Channel{
				Spec: ChannelSpec{
					Provisioner: &corev1.ObjectReference{
						APIVersion: SchemeGroupVersion.String(),
						Kind:       "ClusterChannelProvisioner",
						Name:       "already-specified",
					},
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			if !tc.nilChannelDefaulter {
				ChannelDefaulterSingleton = NewChannelDefaulter(zap.NewNop())
				defer func() { ChannelDefaulterSingleton = nil }()
			}
			if tc.def != nil {
				ChannelDefaulterSingleton.UpdateConfigMap(&corev1.ConfigMap{
					Data: map[string]string{
						channelDefaulterKey: *tc.def,
					},
				})
			}
			tc.initial.SetDefaults()
			if diff := cmp.Diff(tc.expected, tc.initial); diff != "" {
				t.Fatalf("Unexpected defaults (-want, +got): %s", diff)
			}
		})
	}
}
