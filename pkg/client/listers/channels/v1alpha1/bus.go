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

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// BusLister helps list Buses.
type BusLister interface {
	// List lists all Buses in the indexer.
	List(selector labels.Selector) (ret []*v1alpha1.Bus, err error)
	// Get retrieves the Bus from the index for a given name.
	Get(name string) (*v1alpha1.Bus, error)
	BusListerExpansion
}

// busLister implements the BusLister interface.
type busLister struct {
	indexer cache.Indexer
}

// NewBusLister returns a new BusLister.
func NewBusLister(indexer cache.Indexer) BusLister {
	return &busLister{indexer: indexer}
}

// List lists all Buses in the indexer.
func (s *busLister) List(selector labels.Selector) (ret []*v1alpha1.Bus, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.Bus))
	})
	return ret, err
}

// Get retrieves the Bus from the index for a given name.
func (s *busLister) Get(name string) (*v1alpha1.Bus, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("bus"), name)
	}
	return obj.(*v1alpha1.Bus), nil
}
