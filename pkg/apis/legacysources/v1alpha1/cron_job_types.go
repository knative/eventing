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

package v1alpha1

import (
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:defaulter-gen=true

// CronJobSource is the Schema for the cronjobsources API.
type CronJobSource v1alpha1.CronJobSource

var (
	// Check that we can create OwnerReferences to a CronJobSource.
	_ kmeta.OwnerRefable = (*CronJobSource)(nil)
)

// GetGroupVersionKind returns the GroupVersionKind.
func (s *CronJobSource) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("CronJobSource")
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CronJobSourceList contains a list of CronJobSources.
type CronJobSourceList v1alpha1.CronJobSourceList
