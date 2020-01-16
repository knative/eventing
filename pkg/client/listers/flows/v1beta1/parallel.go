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

// Code generated by lister-gen. DO NOT EDIT.

package v1beta1

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	v1beta1 "knative.dev/eventing/pkg/apis/flows/v1beta1"
)

// ParallelLister helps list Parallels.
type ParallelLister interface {
	// List lists all Parallels in the indexer.
	List(selector labels.Selector) (ret []*v1beta1.Parallel, err error)
	// Parallels returns an object that can list and get Parallels.
	Parallels(namespace string) ParallelNamespaceLister
	ParallelListerExpansion
}

// parallelLister implements the ParallelLister interface.
type parallelLister struct {
	indexer cache.Indexer
}

// NewParallelLister returns a new ParallelLister.
func NewParallelLister(indexer cache.Indexer) ParallelLister {
	return &parallelLister{indexer: indexer}
}

// List lists all Parallels in the indexer.
func (s *parallelLister) List(selector labels.Selector) (ret []*v1beta1.Parallel, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.Parallel))
	})
	return ret, err
}

// Parallels returns an object that can list and get Parallels.
func (s *parallelLister) Parallels(namespace string) ParallelNamespaceLister {
	return parallelNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// ParallelNamespaceLister helps list and get Parallels.
type ParallelNamespaceLister interface {
	// List lists all Parallels in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1beta1.Parallel, err error)
	// Get retrieves the Parallel from the indexer for a given namespace and name.
	Get(name string) (*v1beta1.Parallel, error)
	ParallelNamespaceListerExpansion
}

// parallelNamespaceLister implements the ParallelNamespaceLister
// interface.
type parallelNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all Parallels in the indexer for a given namespace.
func (s parallelNamespaceLister) List(selector labels.Selector) (ret []*v1beta1.Parallel, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.Parallel))
	})
	return ret, err
}

// Get retrieves the Parallel from the indexer for a given namespace and name.
func (s parallelNamespaceLister) Get(name string) (*v1beta1.Parallel, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1beta1.Resource("parallel"), name)
	}
	return obj.(*v1beta1.Parallel), nil
}
