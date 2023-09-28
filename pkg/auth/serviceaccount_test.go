/*
Copyright 2023 The Knative Authors

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

package auth

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/pkg/ptr"
)

func TestGetOIDCServiceAccountNameForResource(t *testing.T) {
	tests := []struct {
		name       string
		gvk        schema.GroupVersionKind
		objectMeta metav1.ObjectMeta
		want       string
	}{
		{
			name: "should return SA name in correct format",
			gvk: schema.GroupVersionKind{
				Group:   "group",
				Version: "version",
				Kind:    "kind",
			},
			objectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
			want: "oidc-group-kind-name",
		},
		{
			name: "should return SA name in lower case",
			gvk:  eventingv1.SchemeGroupVersion.WithKind("Broker"),
			objectMeta: metav1.ObjectMeta{
				Name:      "my-Broker",
				Namespace: "my-Namespace",
			},
			want: "oidc-eventing.knative.dev-broker-my-broker",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetOIDCServiceAccountNameForResource(tt.gvk, tt.objectMeta); got != tt.want {
				t.Errorf("GetServiceAccountName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetOIDCServiceAccountForResource(t *testing.T) {
	gvk := eventingv1.SchemeGroupVersion.WithKind("Broker")
	objectMeta := metav1.ObjectMeta{
		Name:      "my-broker",
		Namespace: "my-namespace",
		UID:       "my-uuid",
	}

	want := v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetOIDCServiceAccountNameForResource(gvk, objectMeta),
			Namespace: "my-namespace",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "eventing.knative.dev/v1",
					Kind:               "Broker",
					Name:               "my-broker",
					UID:                "my-uuid",
					Controller:         ptr.Bool(true),
					BlockOwnerDeletion: ptr.Bool(false),
				},
			},
			Annotations: map[string]string{
				"description": "Service Account for OIDC Authentication for Broker \"my-broker\"",
			},
		},
	}

	got := GetOIDCServiceAccountForResource(gvk, objectMeta)

	if diff := cmp.Diff(*got, want); diff != "" {
		t.Errorf("GetServiceAccount() = %+v, want %+v - diff %s", got, want, diff)
	}
}
