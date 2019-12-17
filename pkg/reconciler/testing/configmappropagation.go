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

package testing

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/configs/v1alpha1"
)

// ConfigMapPropagationOption enables further configuration of an ConfigMapPropagation.
type ConfigMapPropagationOption func(*v1alpha1.ConfigMapPropagation)

// NewConfigMapPropagation creates a ConfigMapPropagation.
func NewConfigMapPropagation(name, namespace string, o ...ConfigMapPropagationOption) *v1alpha1.ConfigMapPropagation {
	cmp := &v1alpha1.ConfigMapPropagation{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	for _, opt := range o {
		opt(cmp)
	}
	cmp.SetDefaults(context.Background())
	return cmp
}

// WithInitConfigMapPropagationConditions initializes the ConfigMapPropagation's conditions.
func WithInitConfigMapPropagationConditions(cmp *v1alpha1.ConfigMapPropagation) {
	cmp.Status.InitializeConditions()
}

func WithConfigMapPropagationDeletionTimestamp(cmp *v1alpha1.ConfigMapPropagation) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	cmp.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithConfigMapPropagationSelector(selector map[string]string) ConfigMapPropagationOption {
	return func(cmp *v1alpha1.ConfigMapPropagation) {
		cmp.Spec.Selector = selector
	}
}

// WithConfigMapPropagationPropagated calls .Status.MarkConfigMapPropagationPropagated on the ConfigMapPropagation.
func WithConfigMapPropagationPropagated(cmp *v1alpha1.ConfigMapPropagation) {
	cmp.Status.MarkConfigMapPropagationPropagated()
}

// WithConfigMapPropagationNotPropagated calls .Status.MarkConfigMapPropagationNotPropagated on the ConfigMapPropagation.
func WithConfigMapPropagationNotPropagated(cmp *v1alpha1.ConfigMapPropagation) {
	cmp.Status.MarkConfigMapPropagationNotPropagated()
}
