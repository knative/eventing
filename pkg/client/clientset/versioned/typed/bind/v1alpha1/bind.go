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

// BindsGetter has a method to return a BindInterface.
// A group's client should implement this interface.
type BindsGetter interface {
	Binds(namespace string) BindInterface
}

// BindInterface has methods to work with Bind resources.
type BindInterface interface {
	Create(*v1alpha1.Bind) (*v1alpha1.Bind, error)
	Update(*v1alpha1.Bind) (*v1alpha1.Bind, error)
	UpdateStatus(*v1alpha1.Bind) (*v1alpha1.Bind, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.Bind, error)
	List(opts v1.ListOptions) (*v1alpha1.BindList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.Bind, err error)
	BindExpansion
}

// binds implements BindInterface
type binds struct {
	client rest.Interface
	ns     string
}

// newBinds returns a Binds
func newBinds(c *ElafrosV1alpha1Client, namespace string) *binds {
	return &binds{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the bind, and returns the corresponding bind object, and an error if there is any.
func (c *binds) Get(name string, options v1.GetOptions) (result *v1alpha1.Bind, err error) {
	result = &v1alpha1.Bind{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("binds").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Binds that match those selectors.
func (c *binds) List(opts v1.ListOptions) (result *v1alpha1.BindList, err error) {
	result = &v1alpha1.BindList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("binds").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested binds.
func (c *binds) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("binds").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a bind and creates it.  Returns the server's representation of the bind, and an error, if there is any.
func (c *binds) Create(bind *v1alpha1.Bind) (result *v1alpha1.Bind, err error) {
	result = &v1alpha1.Bind{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("binds").
		Body(bind).
		Do().
		Into(result)
	return
}

// Update takes the representation of a bind and updates it. Returns the server's representation of the bind, and an error, if there is any.
func (c *binds) Update(bind *v1alpha1.Bind) (result *v1alpha1.Bind, err error) {
	result = &v1alpha1.Bind{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("binds").
		Name(bind.Name).
		Body(bind).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *binds) UpdateStatus(bind *v1alpha1.Bind) (result *v1alpha1.Bind, err error) {
	result = &v1alpha1.Bind{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("binds").
		Name(bind.Name).
		SubResource("status").
		Body(bind).
		Do().
		Into(result)
	return
}

// Delete takes name of the bind and deletes it. Returns an error if one occurs.
func (c *binds) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("binds").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *binds) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("binds").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched bind.
func (c *binds) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.Bind, err error) {
	result = &v1alpha1.Bind{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("binds").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
