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

package testing

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NamespaceOption enables further configuration of a Namespace.
type NamespaceOption func(*corev1.Namespace)

// NewNamespace creates a Namespace with NamespaceOptions
func NewNamespace(name string, o ...NamespaceOption) *corev1.Namespace {
	s := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	for _, opt := range o {
		opt(s)
	}
	return s
}

func WithNamespaceDeleted(n *corev1.Namespace) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	n.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithNamespaceLabeled(labels map[string]string) NamespaceOption {
	return func(n *corev1.Namespace) {
		n.Labels = labels
	}
}
