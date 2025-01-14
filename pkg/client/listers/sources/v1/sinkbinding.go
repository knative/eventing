/*
Copyright 2021 The Knative Authors

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

// Code generated by lister-gen. DO NOT EDIT.

package v1

import (
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/listers"
	"k8s.io/client-go/tools/cache"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
)

// SinkBindingLister helps list SinkBindings.
// All objects returned here must be treated as read-only.
type SinkBindingLister interface {
	// List lists all SinkBindings in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.SinkBinding, err error)
	// SinkBindings returns an object that can list and get SinkBindings.
	SinkBindings(namespace string) SinkBindingNamespaceLister
	SinkBindingListerExpansion
}

// sinkBindingLister implements the SinkBindingLister interface.
type sinkBindingLister struct {
	listers.ResourceIndexer[*v1.SinkBinding]
}

// NewSinkBindingLister returns a new SinkBindingLister.
func NewSinkBindingLister(indexer cache.Indexer) SinkBindingLister {
	return &sinkBindingLister{listers.New[*v1.SinkBinding](indexer, v1.Resource("sinkbinding"))}
}

// SinkBindings returns an object that can list and get SinkBindings.
func (s *sinkBindingLister) SinkBindings(namespace string) SinkBindingNamespaceLister {
	return sinkBindingNamespaceLister{listers.NewNamespaced[*v1.SinkBinding](s.ResourceIndexer, namespace)}
}

// SinkBindingNamespaceLister helps list and get SinkBindings.
// All objects returned here must be treated as read-only.
type SinkBindingNamespaceLister interface {
	// List lists all SinkBindings in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.SinkBinding, err error)
	// Get retrieves the SinkBinding from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1.SinkBinding, error)
	SinkBindingNamespaceListerExpansion
}

// sinkBindingNamespaceLister implements the SinkBindingNamespaceLister
// interface.
type sinkBindingNamespaceLister struct {
	listers.ResourceIndexer[*v1.SinkBinding]
}
