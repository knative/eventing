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
	"context"
	"testing"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client/fake"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/feature"
	rttestingv1 "knative.dev/eventing/pkg/reconciler/testing/v1"
	"knative.dev/pkg/ptr"
	rectesting "knative.dev/pkg/reconciler/testing"
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
			want: "name-oidc-group-kind",
		},
		{
			name: "should return SA name in lower case",
			gvk:  eventingv1.SchemeGroupVersion.WithKind("Broker"),
			objectMeta: metav1.ObjectMeta{
				Name:      "my-Broker",
				Namespace: "my-Namespace",
			},
			want: "my-broker-oidc-eventing.knative.dev-broker",
		},
		{
			name: "long Broker name",
			gvk:  eventingv1.SchemeGroupVersion.WithKind("Broker"),
			objectMeta: metav1.ObjectMeta{
				Name:      "my-loooooooooooooooooooooooooooooooooooooog-Broker",
				Namespace: "my-Namespace",
			},
			want: "my-looooooooooooooooooooooooooo2dfc2a3825b8d82077b0f25518b36884",
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
			Labels: map[string]string{
				OIDCLabelKey: "enabled",
			},
		},
	}

	got := GetOIDCServiceAccountForResource(gvk, objectMeta)

	if diff := cmp.Diff(*got, want); diff != "" {
		t.Errorf("GetServiceAccount() = %+v, want %+v - diff %s", got, want, diff)
	}
}

func TestEnsureOIDCServiceAccountExistsForResource(t *testing.T) {
	ctx, _ := rectesting.SetupFakeContext(t)
	gvk := eventingv1.SchemeGroupVersion.WithKind("Broker")
	objectMeta := metav1.ObjectMeta{
		Name:      "my-broker",
		Namespace: "my-namespace",
		UID:       "my-uuid",
	}

	eventtypes := make([]runtime.Object, 0, 10)
	listers := rttestingv1.NewListers(eventtypes)

	err := EnsureOIDCServiceAccountExistsForResource(ctx, listers.GetServiceAccountLister(), kubeclient.Get(ctx), gvk, objectMeta)
	if err != nil {
		t.Errorf("EnsureOIDCServiceAccountExistsForResource failed: %s", err)

	}
	expected := GetOIDCServiceAccountForResource(gvk, objectMeta)
	sa, err := kubeclient.Get(ctx).CoreV1().ServiceAccounts(objectMeta.Namespace).Get(context.TODO(), expected.Name, metav1.GetOptions{})
	if err != nil {
		t.Errorf("get ServiceAccounts  failed: %s", err)
	}
	if sa == nil || sa.Name != expected.Name {
		t.Errorf("EnsureOIDCServiceAccountExistsForResource create ServiceAccounts  failed: %s", err)
	}
}

func TestSetupOIDCServiceAccount(t *testing.T) {
	ctx, _ := rectesting.SetupFakeContext(t)
	gvk := eventingv1.SchemeGroupVersion.WithKind("Trigger")
	objectMeta := metav1.ObjectMeta{
		Name:      "my-trigger",
		Namespace: "my-namespace",
		UID:       "my-uuid",
	}
	eventtypes := make([]runtime.Object, 0, 10)
	listers := rttestingv1.NewListers(eventtypes)
	trigger := rttestingv1.NewTrigger("my-trigger", "my-namespace", "my-broker")
	expected := GetOIDCServiceAccountForResource(gvk, objectMeta)

	// authentication-oidc feature enabled
	err := SetupOIDCServiceAccount(ctx, feature.Flags{
		feature.OIDCAuthentication: feature.Enabled,
	}, listers.GetServiceAccountLister(), kubeclient.Get(ctx), gvk, objectMeta, &trigger.Status, func(as *duckv1.AuthStatus) {
		trigger.Status.Auth = as
	})

	if err != nil {
		t.Errorf("SetupOIDCServiceAccount  failed: %s", err)
	}
	if trigger.Status.Auth == nil || *trigger.Status.Auth.ServiceAccountName != expected.Name {
		t.Errorf("SetupOIDCServiceAccount setAuthStatus  failed")
	}

	// match OIDCIdentityCreated condition
	matched := false
	for _, condition := range trigger.Status.Conditions {
		if condition.Type == eventingv1.TriggerConditionOIDCIdentityCreated {
			if condition.Reason == "" {
				matched = true
			}
		}
	}
	if !matched {
		t.Errorf("SetupOIDCServiceAccount didn't set TriggerConditionOIDCIdentityCreated Status")
	}

	// authentication-oidc feature disabled, after enabling so service account was created.
	// expected serviceAccount was created when SetupOIDCServiceAccount was called with feature enabled.
	listers = rttestingv1.NewListers([]runtime.Object{expected})

	// Verifies that serviceAccount still exists.
	sa, err := kubeclient.Get(ctx).CoreV1().ServiceAccounts(objectMeta.Namespace).Get(context.TODO(), expected.Name, metav1.GetOptions{})
	if sa == nil || err != nil {
		t.Errorf("ServiceAccount does not exist: %+v: %s", sa, err)
	}
	err = SetupOIDCServiceAccount(ctx, feature.Flags{
		feature.OIDCAuthentication: feature.Disabled,
	}, listers.GetServiceAccountLister(), kubeclient.Get(ctx), gvk, objectMeta, &trigger.Status, func(as *duckv1.AuthStatus) {
		trigger.Status.Auth = as
	})

	if err != nil {
		t.Errorf("SetupOIDCServiceAccount  failed: %s", err)
	}

	// Checks whether the created serviceAccount still exists or not.
	sa, err = kubeclient.Get(ctx).CoreV1().ServiceAccounts(objectMeta.Namespace).Get(context.TODO(), expected.Name, metav1.GetOptions{})
	if sa != nil || err == nil {
		t.Errorf("DeleteOIDCServiceAccountIfExists failed to delete the serviceAccount: %+v", sa)
	}

	if trigger.Status.Auth != nil {
		t.Errorf("SetupOIDCServiceAccount setAuthStatus  failed")
	}

	// match OIDCIdentityCreated condition
	matched = false
	for _, condition := range trigger.Status.Conditions {
		if condition.Type == eventingv1.TriggerConditionOIDCIdentityCreated {
			if condition.Reason == "authentication-oidc feature disabled" {
				matched = true
			}
		}
	}

	if !matched {
		t.Errorf("SetupOIDCServiceAccount didn't set TriggerConditionOIDCIdentityCreated Status")
	}

	// authentication-oidc feature disabled.
	listers = rttestingv1.NewListers(eventtypes)
	err = SetupOIDCServiceAccount(ctx, feature.Flags{
		feature.OIDCAuthentication: feature.Disabled,
	}, listers.GetServiceAccountLister(), kubeclient.Get(ctx), gvk, objectMeta, &trigger.Status, func(as *duckv1.AuthStatus) {
		trigger.Status.Auth = as
	})

	if err != nil {
		t.Errorf("SetupOIDCServiceAccount  failed: %s", err)
	}

	if trigger.Status.Auth != nil {
		t.Errorf("SetupOIDCServiceAccount setAuthStatus  failed")
	}

	// match OIDCIdentityCreated condition
	matched = false
	for _, condition := range trigger.Status.Conditions {
		if condition.Type == eventingv1.TriggerConditionOIDCIdentityCreated {
			if condition.Reason == "authentication-oidc feature disabled" {
				matched = true
			}
		}
	}

	if !matched {
		t.Errorf("SetupOIDCServiceAccount didn't set TriggerConditionOIDCIdentityCreated Status")
	}
}

func TestDeleteOIDCServiceAccountIfExists(t *testing.T) {
	ctx, _ := rectesting.SetupFakeContext(t)
	gvk := eventingv1.SchemeGroupVersion.WithKind("Broker")
	objectMeta := metav1.ObjectMeta{
		Name:      "my-broker-unique",
		Namespace: "my-namespace",
		UID:       "my-uuid",
	}

	expected := GetOIDCServiceAccountForResource(gvk, objectMeta)
	serviceAccount, err := kubeclient.Get(ctx).CoreV1().ServiceAccounts(objectMeta.Namespace).Create(ctx, expected, metav1.CreateOptions{})
	if err != nil {
		t.Errorf("could not create OIDC service account %s/%s for %s: %s", objectMeta.Name, objectMeta.Namespace, gvk.Kind, err)
	}

	listers := rttestingv1.NewListers([]runtime.Object{serviceAccount})

	err = DeleteOIDCServiceAccountIfExists(ctx, listers.GetServiceAccountLister(), kubeclient.Get(ctx), gvk, objectMeta)
	if err != nil {
		t.Errorf("DeleteOIDCServiceAccountIfExists failed: %s", err)
	}

	sa, err := kubeclient.Get(ctx).CoreV1().ServiceAccounts(objectMeta.Namespace).Get(context.TODO(), expected.Name, metav1.GetOptions{})
	if sa != nil || err == nil {
		t.Errorf("DeleteOIDCServiceAccountIfExists failed to delete the serviceAccount: %+v", sa)
	}
}
