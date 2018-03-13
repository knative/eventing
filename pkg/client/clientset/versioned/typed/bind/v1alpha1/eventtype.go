/*
Copyright 2018 Google, Inc. All rights reserved.

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

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/elafros/binding/pkg/apis/bind/v1alpha1"
	scheme "github.com/elafros/binding/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// EventTypesGetter has a method to return a EventTypeInterface.
// A group's client should implement this interface.
type EventTypesGetter interface {
	EventTypes(namespace string) EventTypeInterface
}

// EventTypeInterface has methods to work with EventType resources.
type EventTypeInterface interface {
	Create(*v1alpha1.EventType) (*v1alpha1.EventType, error)
	Update(*v1alpha1.EventType) (*v1alpha1.EventType, error)
	UpdateStatus(*v1alpha1.EventType) (*v1alpha1.EventType, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.EventType, error)
	List(opts v1.ListOptions) (*v1alpha1.EventTypeList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.EventType, err error)
	EventTypeExpansion
}

// eventTypes implements EventTypeInterface
type eventTypes struct {
	client rest.Interface
	ns     string
}

// newEventTypes returns a EventTypes
func newEventTypes(c *ElafrosV1alpha1Client, namespace string) *eventTypes {
	return &eventTypes{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the eventType, and returns the corresponding eventType object, and an error if there is any.
func (c *eventTypes) Get(name string, options v1.GetOptions) (result *v1alpha1.EventType, err error) {
	result = &v1alpha1.EventType{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("eventtypes").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of EventTypes that match those selectors.
func (c *eventTypes) List(opts v1.ListOptions) (result *v1alpha1.EventTypeList, err error) {
	result = &v1alpha1.EventTypeList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("eventtypes").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested eventTypes.
func (c *eventTypes) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("eventtypes").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a eventType and creates it.  Returns the server's representation of the eventType, and an error, if there is any.
func (c *eventTypes) Create(eventType *v1alpha1.EventType) (result *v1alpha1.EventType, err error) {
	result = &v1alpha1.EventType{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("eventtypes").
		Body(eventType).
		Do().
		Into(result)
	return
}

// Update takes the representation of a eventType and updates it. Returns the server's representation of the eventType, and an error, if there is any.
func (c *eventTypes) Update(eventType *v1alpha1.EventType) (result *v1alpha1.EventType, err error) {
	result = &v1alpha1.EventType{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("eventtypes").
		Name(eventType.Name).
		Body(eventType).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *eventTypes) UpdateStatus(eventType *v1alpha1.EventType) (result *v1alpha1.EventType, err error) {
	result = &v1alpha1.EventType{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("eventtypes").
		Name(eventType.Name).
		SubResource("status").
		Body(eventType).
		Do().
		Into(result)
	return
}

// Delete takes name of the eventType and deletes it. Returns an error if one occurs.
func (c *eventTypes) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("eventtypes").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *eventTypes) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("eventtypes").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched eventType.
func (c *eventTypes) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.EventType, err error) {
	result = &v1alpha1.EventType{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("eventtypes").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
