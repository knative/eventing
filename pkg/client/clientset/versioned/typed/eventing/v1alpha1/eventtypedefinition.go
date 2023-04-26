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

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
	v1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	scheme "knative.dev/eventing/pkg/client/clientset/versioned/scheme"
)

// EventTypeDefinitionsGetter has a method to return a EventTypeDefinitionInterface.
// A group's client should implement this interface.
type EventTypeDefinitionsGetter interface {
	EventTypeDefinitions(namespace string) EventTypeDefinitionInterface
}

// EventTypeDefinitionInterface has methods to work with EventTypeDefinition resources.
type EventTypeDefinitionInterface interface {
	Create(ctx context.Context, eventTypeDefinition *v1alpha1.EventTypeDefinition, opts v1.CreateOptions) (*v1alpha1.EventTypeDefinition, error)
	Update(ctx context.Context, eventTypeDefinition *v1alpha1.EventTypeDefinition, opts v1.UpdateOptions) (*v1alpha1.EventTypeDefinition, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.EventTypeDefinition, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.EventTypeDefinitionList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.EventTypeDefinition, err error)
	EventTypeDefinitionExpansion
}

// eventTypeDefinitions implements EventTypeDefinitionInterface
type eventTypeDefinitions struct {
	client rest.Interface
	ns     string
}

// newEventTypeDefinitions returns a EventTypeDefinitions
func newEventTypeDefinitions(c *EventingV1alpha1Client, namespace string) *eventTypeDefinitions {
	return &eventTypeDefinitions{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the eventTypeDefinition, and returns the corresponding eventTypeDefinition object, and an error if there is any.
func (c *eventTypeDefinitions) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.EventTypeDefinition, err error) {
	result = &v1alpha1.EventTypeDefinition{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("eventtypedefinitions").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of EventTypeDefinitions that match those selectors.
func (c *eventTypeDefinitions) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.EventTypeDefinitionList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.EventTypeDefinitionList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("eventtypedefinitions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested eventTypeDefinitions.
func (c *eventTypeDefinitions) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("eventtypedefinitions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a eventTypeDefinition and creates it.  Returns the server's representation of the eventTypeDefinition, and an error, if there is any.
func (c *eventTypeDefinitions) Create(ctx context.Context, eventTypeDefinition *v1alpha1.EventTypeDefinition, opts v1.CreateOptions) (result *v1alpha1.EventTypeDefinition, err error) {
	result = &v1alpha1.EventTypeDefinition{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("eventtypedefinitions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(eventTypeDefinition).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a eventTypeDefinition and updates it. Returns the server's representation of the eventTypeDefinition, and an error, if there is any.
func (c *eventTypeDefinitions) Update(ctx context.Context, eventTypeDefinition *v1alpha1.EventTypeDefinition, opts v1.UpdateOptions) (result *v1alpha1.EventTypeDefinition, err error) {
	result = &v1alpha1.EventTypeDefinition{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("eventtypedefinitions").
		Name(eventTypeDefinition.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(eventTypeDefinition).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the eventTypeDefinition and deletes it. Returns an error if one occurs.
func (c *eventTypeDefinitions) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("eventtypedefinitions").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *eventTypeDefinitions) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("eventtypedefinitions").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched eventTypeDefinition.
func (c *eventTypeDefinitions) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.EventTypeDefinition, err error) {
	result = &v1alpha1.EventTypeDefinition{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("eventtypedefinitions").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
