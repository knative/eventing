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

package resolver

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"knative.dev/pkg/configmap"
	. "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/tracker"

	"knative.dev/eventing/pkg/apis/feature"
)

func TestMappingResolver(t *testing.T) {
	testingNamespace := "testns"

	tests := map[string]struct {
		enabled     bool
		ref         *corev1.ObjectReference
		cm          *corev1.ConfigMap
		wantHandled bool
		wantURI     string
		wantErr     bool
	}{
		"experimental feature not enabled": {
			enabled:     false,
			cm:          &corev1.ConfigMap{},
			wantHandled: false,
		},
		"enabled, not handled": {
			enabled:     true,
			ref:         serviceObjRef(testingNamespace),
			cm:          &corev1.ConfigMap{},
			wantHandled: false,
		},
		"enabled, not handled, not empty configmap": {
			enabled: true,
			ref:     serviceObjRef(testingNamespace),
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "config",
					Namespace: testingNamespace,
				},
				Data: map[string]string{
					"Pod.v1": "https://addressable-pod.{{ .SystemNamespace }}.svc.cluster.local/{{ .Name }}",
				},
			},
			wantHandled: false,
		},
		"enabled, handled": {
			enabled: true,
			ref:     serviceObjRef(testingNamespace),
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "config-kreference-mapping",
					Namespace: testingNamespace,
				},
				Data: map[string]string{
					"Service.v1": "https://{{ .Name }}.{{ .Namespace }}.svc.cluster.local/{{ .UID }}",
				},
			},
			wantHandled: true,
			wantURI:     "https://aservice.testns.svc.cluster.local/e23097c8-15a8-487b-b5a3-76fdc4a48c46",
		},
		"enabled, handled, with error (broken template)": {
			enabled: true,
			ref:     serviceObjRef(testingNamespace),
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "config-kreference-mapping",
					Namespace: testingNamespace,
				},
				Data: map[string]string{
					"Service.v1": "https://{{ .DoesNotExist }}.{{ .Namespace }}.svc.cluster.local",
				},
			},
			wantHandled: true,
			wantErr:     true,
		},
		"enabled, handled, with error (does not resolve to a value URL)": {
			enabled: true,
			ref:     serviceObjRef(testingNamespace),
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "config-kreference-mapping",
					Namespace: testingNamespace,
				},
				Data: map[string]string{
					"Service.v1": "%",
				},
			},
			wantHandled: true,
			wantErr:     true,
		},
	}

	for n, tc := range tests {
		tc := tc

		t.Run(n, func(t *testing.T) {
			ctx, _ := SetupFakeContext(t)
			mw := &configmap.ManualWatcher{Namespace: testingNamespace}
			track := tracker.New(func(types.NamespacedName) {}, 0)

			if tc.enabled {
				ctx = feature.ToContext(ctx, feature.Flags{feature.KReferenceMapping: feature.Enabled})
			}

			resolver := NewMappingResolver(ctx, mw, track)
			mw.OnChange(tc.cm)

			handled, uri, err := resolver.MappingURIFromObjectReference(ctx, tc.ref)
			if tc.wantHandled != handled {
				t.Errorf("Unexpected handled value. Got %t, want  %t", handled, tc.wantHandled)
			}

			if handled {
				suri := ""
				if uri != nil {
					suri = uri.String()
				}

				if tc.wantURI != uri.String() {
					t.Errorf("Unexpected URI. Got %s, want %s", suri, tc.wantURI)
				}

				gotErr := err != nil
				if tc.wantErr != gotErr {
					t.Errorf("Unexpected error condition. Got %t, want %t (err: %v)", gotErr, tc.wantErr, err)
				}
			}

		})
	}
}

func serviceObjRef(ns string) *corev1.ObjectReference {
	return &corev1.ObjectReference{
		Kind:       "Service",
		Name:       "aservice",
		APIVersion: "v1",
		Namespace:  ns,
		UID:        "e23097c8-15a8-487b-b5a3-76fdc4a48c46",
	}
}
