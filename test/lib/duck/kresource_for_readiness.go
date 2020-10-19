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

// This file contains functions which check resources until they
// get into the state desired by the caller or time out.

package duck

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

var _ apis.Listable = (*kresourceForReadiness)(nil)

// kresourceForReadiness is a duckv1beta1.KResource, except that it includes all the additional
// fields needed for readiness checking of all the types supported in isResourceReady().
type kresourceForReadiness struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status kresourceForReadinessStatus `json:"status"`
}

type kresourceForReadinessStatus struct {
	duckv1beta1.Status

	// Phase is added to for corev1.Pod readiness checks.
	Phase string `json:"phase,omitempty"`
}

func (k *kresourceForReadiness) DeepCopyObject() runtime.Object {
	if c := k.DeepCopy(); c != nil {
		return k
	}
	return nil
}

func (k *kresourceForReadiness) GetListType() runtime.Object {
	return &kresourceForReadinessList{}
}

type kresourceForReadinessList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []kresourceForReadiness `json:"items"`
}

func (k *kresourceForReadinessList) DeepCopyObject() runtime.Object {
	if c := k.DeepCopy(); c != nil {
		return k
	}
	return nil
}
