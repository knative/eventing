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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Routable is the schema for the routable portion of the spec
// section of the resource.
type Routable struct {
	// This is the router specification.
	Router RoutableSpec `json:"router,omitempty"`
}

// RoutableSpec defines a router
type RoutableSpec struct {
	// Ref is a reference to the router service
	Ref *corev1.ObjectReference `json:"ref,omitempty"`

	// +optional
	RouterURI string `json:"routerURI,omitempty"`

	// Arguments defines the arguments to pass to the Router
	// +optional
	Arguments *runtime.RawExtension `json:"arguments,omitempty"`
}
