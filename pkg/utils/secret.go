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
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// CopySecret will copy a secret from one namespace into another.
// If a ServiceAccount name is provided then it'll add it as a PullSecret to
// it.
// It'll either return a pointer to the new Secret or and error indicating
// why it couldn't do it.
func CopySecret(corev1Input clientcorev1.CoreV1Interface, srcNS string, srcSecretName string, tgtNS string, svcAccount string) (*corev1.Secret, error) {
	tgtNamespaceSvcAcct := corev1Input.ServiceAccounts(tgtNS)
	srcSecrets := corev1Input.Secrets(srcNS)
	tgtNamespaceSecrets := corev1Input.Secrets(tgtNS)

	// First try to find the secret we're supposed to copy
	srcSecret, err := srcSecrets.Get(context.Background(), srcSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// check for nil source secret
	if srcSecret == nil {
		return nil, errors.New("error copying secret; there is no error but secret is nil")
	}

	// Found the secret, so now make a copy in our new namespace
	newSecret, err := tgtNamespaceSecrets.Create(
		context.Background(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: srcSecretName,
			},
			Data: srcSecret.Data,
			Type: srcSecret.Type,
		},
		metav1.CreateOptions{})

	// If the secret already exists then that's ok - may have already been created
	if err != nil && !apierrs.IsAlreadyExists(err) {
		return nil, fmt.Errorf("error copying the Secret: %s", err)
	}

	_, err = tgtNamespaceSvcAcct.Patch(context.Background(), svcAccount, types.StrategicMergePatchType,
		[]byte(`{"imagePullSecrets":[{"name":"`+srcSecretName+`"}]}`), metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("patch failed on NS/SA (%s/%s): %s",
			tgtNS, srcSecretName, err)
	}
	return newSecret, nil
}
