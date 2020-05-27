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

package utils

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"knative.dev/eventing/pkg/reconciler/mtnamespace/resources"
)

const (
	srcNamespace   = "src-namespace"
	tgtNamespace   = "tgt-namespace"
	svcAccountName = "svc-account"
	pullSecretName = "pull-secret-name"
)

type KubernetesAPI struct {
	Client kubernetes.Interface
}

func TestCopySecret(t *testing.T) {

	api := &KubernetesAPI{
		Client: fake.NewSimpleClientset(),
	}

	testCases := map[string]struct {
		corev1Input   clientcorev1.CoreV1Interface
		srcNS         string
		srcSecretName string
		tgtNS         string
		svcAccount    string
		errorExpected bool
	}{
		"copy secret": {
			corev1Input:   api.Client.CoreV1(),
			srcNS:         srcNamespace,
			srcSecretName: pullSecretName,
			tgtNS:         tgtNamespace,
			svcAccount:    svcAccountName,
			errorExpected: false,
		},
		"tgt namespace does not exist": {
			corev1Input:   api.Client.CoreV1(),
			srcNS:         srcNamespace,
			srcSecretName: pullSecretName,
			tgtNS:         "does-not-exist",
			svcAccount:    svcAccountName,
			errorExpected: true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			// set up src namespace secret
			srcNamespaceSecrets := tc.corev1Input.Secrets(srcNamespace)
			_, secretCreateError := srcNamespaceSecrets.Create(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: pullSecretName},
			})
			if secretCreateError != nil {
				t.Errorf("error creating secret resources for test case: %s", secretCreateError)

			}

			// set up tgt namespace and service account to copy secret into.
			tgtNamespaceServiceAccts := tc.corev1Input.ServiceAccounts(tgtNamespace)
			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: tgtNamespace,
				}}
			_, saCreateError := tgtNamespaceServiceAccts.Create(makeServiceAccount(namespace, svcAccountName))
			if saCreateError != nil {
				t.Errorf("error creating service account resources for test case %s", saCreateError)
			}

			// try copying the secret
			copiedSecret, err := CopySecret(tc.corev1Input, tc.srcNS, tc.srcSecretName, tc.tgtNS, tc.svcAccount)
			if !tc.errorExpected {
				if err != nil {
					t.Errorf("unexpected error on copying secret %v", err)
				}
				if copiedSecret.Name != tc.srcSecretName {
					t.Errorf("Expected %s, actually %s", tc.srcSecretName, copiedSecret.Name)
				}
				if copiedSecret.Namespace != tc.tgtNS {
					t.Errorf("Expected %s, actually %s", tc.tgtNS, copiedSecret.Namespace)
				}
			} else {
				if err == nil {
					t.Errorf("expected error on copying secret, none found")
				}
			}

			// clean up after test
			tc.corev1Input.ServiceAccounts(tgtNamespace).Delete(svcAccountName, &metav1.DeleteOptions{})
			tc.corev1Input.Secrets(srcNamespace).Delete(pullSecretName, &metav1.DeleteOptions{})
			tc.corev1Input.Secrets(tc.tgtNS).Delete(pullSecretName, &metav1.DeleteOptions{})
		})
	}
}

// makeServiceAccount creates a ServiceAccount object for the Namespace 'ns'.
func makeServiceAccount(namespace *corev1.Namespace, name string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(namespace.GetObjectMeta(), schema.GroupVersionKind{
					Group:   corev1.SchemeGroupVersion.Group,
					Version: corev1.SchemeGroupVersion.Version,
					Kind:    "Namespace",
				}),
			},
			Namespace: namespace.Name,
			Name:      name,
			Labels:    resources.OwnedLabels(),
		},
	}
}
