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

package channeldefaulter

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/google/go-cmp/cmp"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

var (
	def = &corev1.ObjectReference{
		APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
		Kind:       "ClusterChannelProvisioner",
		Name:       "test-channel-provisioner",
	}
)

func TestChannelDefaulter_setDefaultProvider(t *testing.T) {
	testCases := map[string]struct {
		nilChannelDefaulter bool
		def                 *corev1.ObjectReference
		spec                *eventingv1alpha1.ChannelSpec
		expected            *eventingv1alpha1.ChannelSpec
	}{
		"nil channel defaulter": {
			nilChannelDefaulter: true,
		},
		"nil spec": {},
		"no default set": {
			spec: &eventingv1alpha1.ChannelSpec{},
			expected: &eventingv1alpha1.ChannelSpec{
				Provisioner: nil,
			},
		},
		"defaulted": {
			def:  def,
			spec: &eventingv1alpha1.ChannelSpec{},
			expected: &eventingv1alpha1.ChannelSpec{
				Provisioner: def,
			},
		},
		"defaulted - removing arguments": {
			def: def,
			spec: &eventingv1alpha1.ChannelSpec{
				Arguments: &runtime.RawExtension{
					Object: &corev1.ObjectReference{
						Name: "will-be-removed",
					},
				},
			},
			expected: &eventingv1alpha1.ChannelSpec{
				Provisioner: def,
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			var cd *ChannelDefaulter
			if !tc.nilChannelDefaulter {
				cd = New(zap.NewNop())
			}
			if tc.def != nil {
				cd.setConfig(tc.def)
			}
			spec := tc.spec
			cd.SetChannelProvisioner(spec)
			if diff := cmp.Diff(tc.expected, spec); diff != "" {
				t.Fatalf("Unexpetec result (-want, +got): %s", diff)
			}
		})
	}
}

func TestChannelDefaulter_UpdateConfigMap(t *testing.T) {
	testCases := map[string]struct {
		initialConfig        *corev1.ConfigMap
		expectedAfterInitial *eventingv1alpha1.ChannelSpec
		updatedConfig        *corev1.ConfigMap
		expectedAfterUpdate  *eventingv1alpha1.ChannelSpec
	}{
		"nil config map": {
			expectedAfterInitial: &eventingv1alpha1.ChannelSpec{},
			expectedAfterUpdate:  &eventingv1alpha1.ChannelSpec{},
		},
		"key missing": {
			initialConfig: &corev1.ConfigMap{
				Data: map[string]string{
					channelDefaulterKey: def.Name,
				},
			},
			expectedAfterInitial: &eventingv1alpha1.ChannelSpec{
				Provisioner: def,
			},
			updatedConfig: &corev1.ConfigMap{},
			expectedAfterUpdate: &eventingv1alpha1.ChannelSpec{
				Provisioner: def,
			},
		},
		"default is empty string": {
			initialConfig: &corev1.ConfigMap{
				Data: map[string]string{
					channelDefaulterKey: def.Name,
				},
			},
			expectedAfterInitial: &eventingv1alpha1.ChannelSpec{
				Provisioner: def,
			},
			updatedConfig: &corev1.ConfigMap{
				Data: map[string]string{
					channelDefaulterKey: "",
				},
			},
			expectedAfterUpdate: &eventingv1alpha1.ChannelSpec{},
		},
		"update to same provisioner": {
			initialConfig: &corev1.ConfigMap{
				Data: map[string]string{
					channelDefaulterKey: def.Name,
				},
			},
			expectedAfterInitial: &eventingv1alpha1.ChannelSpec{
				Provisioner: def,
			},
			updatedConfig: &corev1.ConfigMap{
				Data: map[string]string{
					channelDefaulterKey: def.Name,
				},
			},
			expectedAfterUpdate: &eventingv1alpha1.ChannelSpec{
				Provisioner: def,
			},
		},
		"update to different provisioner": {
			initialConfig: &corev1.ConfigMap{
				Data: map[string]string{
					channelDefaulterKey: def.Name,
				},
			},
			expectedAfterInitial: &eventingv1alpha1.ChannelSpec{
				Provisioner: def,
			},
			updatedConfig: &corev1.ConfigMap{
				Data: map[string]string{
					channelDefaulterKey: "some-other-name",
				},
			},
			expectedAfterUpdate: &eventingv1alpha1.ChannelSpec{
				Provisioner: &corev1.ObjectReference{
					APIVersion: def.APIVersion,
					Kind:       def.Kind,
					Name:       "some-other-name",
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			cd := New(zap.NewNop())
			cd.UpdateConfigMap(tc.initialConfig)

			initialSpec := &eventingv1alpha1.ChannelSpec{}
			cd.SetChannelProvisioner(initialSpec)
			if diff := cmp.Diff(tc.expectedAfterInitial, initialSpec); diff != "" {
				t.Fatalf("Unexpected difference after intial configMap update (-want, +got): %s", diff)
			}

			cd.UpdateConfigMap(tc.updatedConfig)
			updateSpec := &eventingv1alpha1.ChannelSpec{}
			cd.SetChannelProvisioner(updateSpec)
			if diff := cmp.Diff(tc.expectedAfterUpdate, updateSpec); diff != "" {
				t.Fatalf("Unexpected difference after update configMap update (-want, +got): %s", diff)
			}
		})
	}
}
