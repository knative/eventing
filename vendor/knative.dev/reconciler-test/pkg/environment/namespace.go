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
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
)

const (
	interval = 1 * time.Second
	timeout  = 45 * time.Second
)

// CreateNamespaceIfNeeded creates a new namespace if it does not exist.
func (me *MagicEnvironment) CreateNamespaceIfNeeded() error {
	c := kubeclient.Get(me.c)
	nsSpec, err := c.CoreV1().Namespaces().Get(context.Background(), me.namespace, metav1.GetOptions{})

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		// Namespace was not found, try to create it.

		nsSpec = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: me.namespace}}
		nsSpec, err = c.CoreV1().Namespaces().Create(context.Background(), nsSpec, metav1.CreateOptions{})

		if err != nil {
			return fmt.Errorf("failed to create Namespace: %s; %v", me.namespace, err)
		}
		me.namespaceCreated = true

		// https://github.com/kubernetes/kubernetes/issues/66689
		// We can only start creating pods after the default ServiceAccount is created by the kube-controller-manager.
		if err := wait.PollImmediate(interval, timeout, func() (bool, error) {
			sas := c.CoreV1().ServiceAccounts(me.namespace)
			if _, err := sas.Get(context.Background(), "default", metav1.GetOptions{}); err == nil {
				return true, nil
			}
			return false, nil
		}); err != nil {
			return fmt.Errorf("the default ServiceAccount was not created for the Namespace: %s", me.namespace)
		}
	}
	return nil
}

func (me *MagicEnvironment) DeleteNamespaceIfNeeded() error {
	if me.namespaceCreated {
		c := kubeclient.Get(me.c)

		_, err := c.CoreV1().Namespaces().Get(context.Background(), me.namespace, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			me.namespaceCreated = false
			return nil
		} else if err != nil {
			return err
		}

		if err := c.CoreV1().Namespaces().Delete(context.Background(), me.namespace, metav1.DeleteOptions{}); err != nil {
			return err
		}
		me.namespaceCreated = false
	}

	return nil
}
