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
package apiserver

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
)

type ResourceWatch struct {
	// GVR the group version resource of the resource to watch.
	GVR schema.GroupVersionResource `json:"gvr"`

	// LabelSelector filters this source to objects to those resources pass the
	// label selector.
	// +optional
	LabelSelector string `json:"selector,omitempty"`
}

type Config struct {
	// Namespaces specifies the namespaces where Resources[] exist.
	// +required
	Namespaces []string `json:"namespaces"`

	// AllNamespaces indicates whether this source is watching all
	// existing namespaces
	AllNamespaces bool `json:"allNamespaces"`

	// Resource is the resource this source will track and send related
	// lifecycle events from the Kubernetes ApiServer.
	// +required
	Resources []ResourceWatch `json:"resources"`

	// ResourceOwner is an additional filter to only track resources that are
	// owned by a specific resource type. If ResourceOwner matches Resources[n]
	// then Resources[n] is allowed to pass the ResourceOwner filter.
	// +optional
	ResourceOwner *v1.APIVersionKind `json:"owner,omitempty"`

	// EventMode controls the format of the event.
	// `Reference` sends a dataref event type for the resource under watch.
	// `Resource` send the full resource lifecycle event.
	// Defaults to `Reference`
	// +optional
	EventMode string `json:"mode,omitempty"`
}
