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

package environment

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeclient "knative.dev/pkg/client/injection/kube/client"

	"knative.dev/reconciler-test/pkg/milestone"
)

type namespaceKey struct{}

func withNamespace(ctx context.Context, namespace string) context.Context {
	return context.WithValue(ctx, namespaceKey{}, namespace)
}

func getNamespace(ctx context.Context) string {
	ns := ctx.Value(namespaceKey{})
	if ns == nil {
		return ""
	}
	return ns.(string)
}

// CreateNamespaceIfNeeded creates a new namespace if it does not exist.
func (mr *MagicEnvironment) CreateNamespaceIfNeeded() error {
	c := kubeclient.Get(mr.c)
	_, err := c.CoreV1().Namespaces().Get(context.Background(), mr.namespace, metav1.GetOptions{})

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		// Namespace was not found, try to create it.

		nsSpec := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:        mr.namespace,
				Annotations: map[string]string{},
			},
		}

		if cfg := GetIstioConfig(mr.c); cfg.Enabled {
			withIstioNamespaceLabel(nsSpec)
		}

		_, err = c.CoreV1().Namespaces().Create(context.Background(), nsSpec, metav1.CreateOptions{})

		if err != nil {
			return fmt.Errorf("failed to create Namespace: %s; %v", mr.namespace, err)
		}
		mr.namespaceCreated = true
		mr.milestones.NamespaceCreated(mr.namespace)

		interval, timeout := PollTimingsFromContext(mr.c)

		// https://github.com/kubernetes/kubernetes/issues/66689
		// We can only start creating pods after the default ServiceAccount is created by the kube-controller-manager.
		var sa *corev1.ServiceAccount
		if err := wait.PollImmediate(interval, timeout, func() (bool, error) {
			sas := c.CoreV1().ServiceAccounts(mr.namespace)
			if sa, err = sas.Get(context.Background(), "default", metav1.GetOptions{}); err == nil {
				return true, nil
			}
			return false, nil
		}); err != nil {
			return fmt.Errorf("the default ServiceAccount was not created for the Namespace: %s", mr.namespace)
		}

		srcSecret, err := c.CoreV1().Secrets(mr.imagePullSecretNamespace).Get(context.Background(), mr.imagePullSecretName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				// Image pull secret doesn't exist, so no need to continue
				return nil
			}
			return fmt.Errorf("error retrieving %s/%s secret: %s", mr.imagePullSecretNamespace, mr.imagePullSecretName, err)
		}

		// If image pull secret exists in the default namespace, copy it over to the new namespace
		_, err = c.CoreV1().Secrets(mr.namespace).Create(
			context.Background(),
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: mr.imagePullSecretName,
				},
				Data: srcSecret.Data,
				Type: srcSecret.Type,
			},
			metav1.CreateOptions{})

		if err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("error copying the image pull Secret: %s", err)
		}

		_, err = c.CoreV1().ServiceAccounts(mr.namespace).Patch(context.Background(), sa.Name, types.StrategicMergePatchType,
			[]byte(`{"imagePullSecrets":[{"name":"`+mr.imagePullSecretName+`"}]}`), metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("patch failed on NS/SA (%s/%s): %s", mr.namespace, sa.Name, err)
		}
	}
	return nil
}

func (mr *MagicEnvironment) DeleteNamespaceIfNeeded(result milestone.Result) error {
	if (result.Failed() && !mr.teardownOnFail) || !mr.namespaceCreated {
		return nil
	}

	c := kubeclient.Get(mr.c)

	_, err := c.CoreV1().Namespaces().Get(context.Background(), mr.namespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		mr.namespaceCreated = false
		return nil
	} else if err != nil {
		return err
	}

	if err := c.CoreV1().Namespaces().Delete(context.Background(), mr.namespace, metav1.DeleteOptions{}); err != nil {
		return err
	}
	mr.namespaceCreated = false
	mr.milestones.NamespaceDeleted(mr.namespace)

	return nil
}
