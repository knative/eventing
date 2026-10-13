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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

// controllerFilter by apiVersion and/or kind
type controllerFilter struct {
	apiVersion string
	kind       string
	delegate   cache.ReflectorStore
}

var _ cache.ReflectorStore = (*controllerFilter)(nil)
var _ cache.ReflectorBookmarkStore = (*controllerFilter)(nil)

// Implements Store

func (c *controllerFilter) Add(obj interface{}) error {
	if c.filtered(obj) {
		return nil
	}

	return c.delegate.Add(obj)
}

func (c *controllerFilter) Update(obj interface{}) error {
	if c.filtered(obj) {
		return nil
	}

	return c.delegate.Update(obj)
}

func (c *controllerFilter) Delete(obj interface{}) error {
	if c.filtered(obj) {
		return nil
	}

	return c.delegate.Delete(obj)
}

func (c *controllerFilter) filtered(obj interface{}) bool {
	u := obj.(*unstructured.Unstructured)
	controller := metav1.GetControllerOf(u)
	return controller == nil || (c.apiVersion != "" && c.apiVersion != controller.APIVersion) ||
		(c.kind != "" && c.kind != controller.Kind)
}

// The remaining cache methods do not emit events; the adapter does not retain objects.

func (c *controllerFilter) List() []interface{} {
	return nil
}

func (c *controllerFilter) ListKeys() []string {
	return nil
}

func (c *controllerFilter) Get(obj interface{}) (item interface{}, exists bool, err error) {
	return nil, false, nil
}

func (c *controllerFilter) GetByKey(key string) (item interface{}, exists bool, err error) {
	return nil, false, nil
}

func (c *controllerFilter) Replace([]interface{}, string) error {
	return nil
}

func (c *controllerFilter) Resync() error {
	return nil
}

func (c *controllerFilter) Bookmark(resourceVersion string) error {
	return nil
}
