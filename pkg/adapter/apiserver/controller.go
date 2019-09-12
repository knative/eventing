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

package apiserver

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

// filter controller by apiVersion and/or kind
type controller struct {
	apiVersion string
	kind       string
	delegate   cache.Store
}

var _ cache.Store = (*controller)(nil)

// Implements Store

func (c *controller) Add(obj interface{}) error {
	if c.filtered(obj) {
		return nil
	}

	return c.delegate.Add(obj)
}

func (c *controller) Update(obj interface{}) error {
	if c.filtered(obj) {
		return nil
	}

	return c.delegate.Update(obj)
}

func (c *controller) Delete(obj interface{}) error {
	if c.filtered(obj) {
		return nil
	}

	return c.delegate.Delete(obj)
}

func (c *controller) filtered(obj interface{}) bool {
	u := obj.(*unstructured.Unstructured)
	controller := metav1.GetControllerOf(u)
	return controller == nil || (c.apiVersion != "" && c.apiVersion != controller.APIVersion) ||
		(c.kind != "" && c.kind != controller.Kind)
}

// Stub cache.Store impl

// Implements cache.Store
func (c *controller) List() []interface{} {
	return nil
}

// Implements cache.Store
func (c *controller) ListKeys() []string {
	return nil
}

// Implements cache.Store
func (c *controller) Get(obj interface{}) (item interface{}, exists bool, err error) {
	return nil, false, nil
}

// Implements cache.Store
func (c *controller) GetByKey(key string) (item interface{}, exists bool, err error) {
	return nil, false, nil
}

// Implements cache.Store
func (c *controller) Replace([]interface{}, string) error {
	return nil
}

// Implements cache.Store
func (c *controller) Resync() error {
	return nil
}
