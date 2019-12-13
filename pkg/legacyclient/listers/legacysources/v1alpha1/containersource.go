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

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	v1alpha1 "knative.dev/eventing/pkg/apis/legacysources/v1alpha1"
)

// ContainerSourceLister helps list ContainerSources.
type ContainerSourceLister interface {
	// List lists all ContainerSources in the indexer.
	List(selector labels.Selector) (ret []*v1alpha1.ContainerSource, err error)
	// ContainerSources returns an object that can list and get ContainerSources.
	ContainerSources(namespace string) ContainerSourceNamespaceLister
	ContainerSourceListerExpansion
}

// containerSourceLister implements the ContainerSourceLister interface.
type containerSourceLister struct {
	indexer cache.Indexer
}

// NewContainerSourceLister returns a new ContainerSourceLister.
func NewContainerSourceLister(indexer cache.Indexer) ContainerSourceLister {
	return &containerSourceLister{indexer: indexer}
}

// List lists all ContainerSources in the indexer.
func (s *containerSourceLister) List(selector labels.Selector) (ret []*v1alpha1.ContainerSource, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.ContainerSource))
	})
	return ret, err
}

// ContainerSources returns an object that can list and get ContainerSources.
func (s *containerSourceLister) ContainerSources(namespace string) ContainerSourceNamespaceLister {
	return containerSourceNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// ContainerSourceNamespaceLister helps list and get ContainerSources.
type ContainerSourceNamespaceLister interface {
	// List lists all ContainerSources in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1alpha1.ContainerSource, err error)
	// Get retrieves the ContainerSource from the indexer for a given namespace and name.
	Get(name string) (*v1alpha1.ContainerSource, error)
	ContainerSourceNamespaceListerExpansion
}

// containerSourceNamespaceLister implements the ContainerSourceNamespaceLister
// interface.
type containerSourceNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all ContainerSources in the indexer for a given namespace.
func (s containerSourceNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.ContainerSource, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.ContainerSource))
	})
	return ret, err
}

// Get retrieves the ContainerSource from the indexer for a given namespace and name.
func (s containerSourceNamespaceLister) Get(name string) (*v1alpha1.ContainerSource, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("containersource"), name)
	}
	return obj.(*v1alpha1.ContainerSource), nil
}
