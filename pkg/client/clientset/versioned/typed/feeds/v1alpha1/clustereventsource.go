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

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	scheme "github.com/knative/eventing/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ClusterEventSourcesGetter has a method to return a ClusterEventSourceInterface.
// A group's client should implement this interface.
type ClusterEventSourcesGetter interface {
	ClusterEventSources(namespace string) ClusterEventSourceInterface
}

// ClusterEventSourceInterface has methods to work with ClusterEventSource resources.
type ClusterEventSourceInterface interface {
	Create(*v1alpha1.ClusterEventSource) (*v1alpha1.ClusterEventSource, error)
	Update(*v1alpha1.ClusterEventSource) (*v1alpha1.ClusterEventSource, error)
	UpdateStatus(*v1alpha1.ClusterEventSource) (*v1alpha1.ClusterEventSource, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.ClusterEventSource, error)
	List(opts v1.ListOptions) (*v1alpha1.ClusterEventSourceList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.ClusterEventSource, err error)
	ClusterEventSourceExpansion
}

// clusterEventSources implements ClusterEventSourceInterface
type clusterEventSources struct {
	client rest.Interface
	ns     string
}

// newClusterEventSources returns a ClusterEventSources
func newClusterEventSources(c *FeedsV1alpha1Client, namespace string) *clusterEventSources {
	return &clusterEventSources{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the clusterEventSource, and returns the corresponding clusterEventSource object, and an error if there is any.
func (c *clusterEventSources) Get(name string, options v1.GetOptions) (result *v1alpha1.ClusterEventSource, err error) {
	result = &v1alpha1.ClusterEventSource{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("clustereventsources").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ClusterEventSources that match those selectors.
func (c *clusterEventSources) List(opts v1.ListOptions) (result *v1alpha1.ClusterEventSourceList, err error) {
	result = &v1alpha1.ClusterEventSourceList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("clustereventsources").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested clusterEventSources.
func (c *clusterEventSources) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("clustereventsources").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a clusterEventSource and creates it.  Returns the server's representation of the clusterEventSource, and an error, if there is any.
func (c *clusterEventSources) Create(clusterEventSource *v1alpha1.ClusterEventSource) (result *v1alpha1.ClusterEventSource, err error) {
	result = &v1alpha1.ClusterEventSource{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("clustereventsources").
		Body(clusterEventSource).
		Do().
		Into(result)
	return
}

// Update takes the representation of a clusterEventSource and updates it. Returns the server's representation of the clusterEventSource, and an error, if there is any.
func (c *clusterEventSources) Update(clusterEventSource *v1alpha1.ClusterEventSource) (result *v1alpha1.ClusterEventSource, err error) {
	result = &v1alpha1.ClusterEventSource{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("clustereventsources").
		Name(clusterEventSource.Name).
		Body(clusterEventSource).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *clusterEventSources) UpdateStatus(clusterEventSource *v1alpha1.ClusterEventSource) (result *v1alpha1.ClusterEventSource, err error) {
	result = &v1alpha1.ClusterEventSource{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("clustereventsources").
		Name(clusterEventSource.Name).
		SubResource("status").
		Body(clusterEventSource).
		Do().
		Into(result)
	return
}

// Delete takes name of the clusterEventSource and deletes it. Returns an error if one occurs.
func (c *clusterEventSources) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("clustereventsources").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *clusterEventSources) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("clustereventsources").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched clusterEventSource.
func (c *clusterEventSources) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.ClusterEventSource, err error) {
	result = &v1alpha1.ClusterEventSource{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("clustereventsources").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
