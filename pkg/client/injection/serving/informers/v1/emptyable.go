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

package service

import v1 "knative.dev/serving/pkg/client/informers/externalversions/serving/v1"

// EmptyableServiceInformer is an "emptyable" version of v1.ServiceInformer.
type EmptyableServiceInformer interface {
	// IsEmpty returns whether the informer is empty.
	IsEmpty() bool

	// GetInternal returns the internal v1.ServiceInformer if the informer is not empty.
	GetInternal() v1.ServiceInformer

	// Run delegates to v1.ServiceInformer if the informer is not empty.
	// Otherwise it does nothing.
	Run(<-chan struct{})

	// HasSynced delegates to v1.ServiceInformer if the informer is not empty.
	// Otherwise it always returns true.
	HasSynced() bool
}
