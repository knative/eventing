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

package config

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "knative.dev/pkg/configmap/testing"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing"
)

func TestNewPingDefaultsConfigFromConfigMap(t *testing.T) {
	_, example := ConfigMapsFromTestFile(t, PingDefaultsConfigName)
	if _, err := NewPingDefaultsConfigFromConfigMap(example); err != nil {
		t.Error("NewPingDefaultsConfigFromMap(example) =", err)
	}
}

func TestNewPingDefaultsConfigFromLegacyConfigMap(t *testing.T) {
	// Using legacy ConfiMap with to be deprecated element.
	_, example := ConfigMapsFromTestFile(t, PingDefaultsConfigName+"-legacy")
	if _, err := NewPingDefaultsConfigFromConfigMap(example); err != nil {
		t.Error("NewPingDefaultsConfigFromMap(example) =", err)
	}
}

func TestPingDefaultsConfiguration(t *testing.T) {
	testCases := []struct {
		name        string
		wantErr     bool
		wantDefault int64
		config      *corev1.ConfigMap
	}{{
		name:        "default config",
		wantErr:     false,
		wantDefault: DefaultDataMaxSize,
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      PingDefaultsConfigName,
			},
			Data: map[string]string{},
		},
	}, {
		name:        "example text",
		wantErr:     false,
		wantDefault: DefaultDataMaxSize,
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      PingDefaultsConfigName,
			},
			Data: map[string]string{
				"_example": `
			################################
			#                              #
			#    EXAMPLE CONFIGURATION     #
			#                              #
			################################

			# Max number of bytes allowed to be sent for message excluding any
			# base64 decoding.  Default is no limit set for data
			data-max-size: 4096
	`,
			},
		},
	}, {
		name:        "legacy example text",
		wantErr:     false,
		wantDefault: DefaultDataMaxSize,
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      PingDefaultsConfigName,
			},
			Data: map[string]string{
				"_example": `
    ################################
    #                              #
    #    EXAMPLE CONFIGURATION     #
    #                              #
    ################################

    # Max number of bytes allowed to be sent for message excluding any
    # base64 decoding.  Default is no limit set for data
    dataMaxSize: 4096
`,
			},
		},
	}, {
		name:        "specific data-max-size",
		wantErr:     false,
		wantDefault: 1337,
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      PingDefaultsConfigName,
			},
			Data: map[string]string{
				"data-max-size": "1337",
			},
		},
	}, {
		name:        "dangling key/value pair",
		wantErr:     true,
		wantDefault: DefaultDataMaxSize,
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      PingDefaultsConfigName,
			},
			Data: map[string]string{
				"data-max-size": "#nothing to see here",
			},
		},
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualDefault, err := NewPingDefaultsConfigFromConfigMap(tc.config)

			if (err != nil) != tc.wantErr {
				t.Fatalf("Test: %q: NewPingDefaultsConfigFromMap() error = %v, wantErr %v", tc.name, err, tc.wantErr)
			}
			if !tc.wantErr {
				// Testing DeepCopy just to increase coverage
				actualDefault = actualDefault.DeepCopy()

				if diff := cmp.Diff(tc.wantDefault, actualDefault.GetPingConfig().DataMaxSize); diff != "" {
					t.Error("unexpected value (-want, +got)", diff)
				}
			}
		})
	}
}
