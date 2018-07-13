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

// ClusterEventTypesGetter has a method to return a ClusterEventTypeInterface.
// A group's client should implement this interface.
type ClusterEventTypesGetter interface {
	ClusterEventTypes(namespace string) ClusterEventTypeInterface
}

// ClusterEventTypeInterface has methods to work with ClusterEventType resources.
type ClusterEventTypeInterface interface {
	Create(*v1alpha1.ClusterEventType) (*v1alpha1.ClusterEventType, error)
	Update(*v1alpha1.ClusterEventType) (*v1alpha1.ClusterEventType, error)
	UpdateStatus(*v1alpha1.ClusterEventType) (*v1alpha1.ClusterEventType, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.ClusterEventType, error)
	List(opts v1.ListOptions) (*v1alpha1.ClusterEventTypeList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.ClusterEventType, err error)
	ClusterEventTypeExpansion
}

// clusterEventTypes implements ClusterEventTypeInterface
type clusterEventTypes struct {
	client rest.Interface
	ns     string
}

// newClusterEventTypes returns a ClusterEventTypes
func newClusterEventTypes(c *FeedsV1alpha1Client, namespace string) *clusterEventTypes {
	return &clusterEventTypes{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the clusterEventType, and returns the corresponding clusterEventType object, and an error if there is any.
func (c *clusterEventTypes) Get(name string, options v1.GetOptions) (result *v1alpha1.ClusterEventType, err error) {
	result = &v1alpha1.ClusterEventType{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("clustereventtypes").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ClusterEventTypes that match those selectors.
func (c *clusterEventTypes) List(opts v1.ListOptions) (result *v1alpha1.ClusterEventTypeList, err error) {
	result = &v1alpha1.ClusterEventTypeList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("clustereventtypes").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested clusterEventTypes.
func (c *clusterEventTypes) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("clustereventtypes").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a clusterEventType and creates it.  Returns the server's representation of the clusterEventType, and an error, if there is any.
func (c *clusterEventTypes) Create(clusterEventType *v1alpha1.ClusterEventType) (result *v1alpha1.ClusterEventType, err error) {
	result = &v1alpha1.ClusterEventType{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("clustereventtypes").
		Body(clusterEventType).
		Do().
		Into(result)
	return
}

// Update takes the representation of a clusterEventType and updates it. Returns the server's representation of the clusterEventType, and an error, if there is any.
func (c *clusterEventTypes) Update(clusterEventType *v1alpha1.ClusterEventType) (result *v1alpha1.ClusterEventType, err error) {
	result = &v1alpha1.ClusterEventType{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("clustereventtypes").
		Name(clusterEventType.Name).
		Body(clusterEventType).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *clusterEventTypes) UpdateStatus(clusterEventType *v1alpha1.ClusterEventType) (result *v1alpha1.ClusterEventType, err error) {
	result = &v1alpha1.ClusterEventType{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("clustereventtypes").
		Name(clusterEventType.Name).
		SubResource("status").
		Body(clusterEventType).
		Do().
		Into(result)
	return
}

// Delete takes name of the clusterEventType and deletes it. Returns an error if one occurs.
func (c *clusterEventTypes) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("clustereventtypes").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *clusterEventTypes) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("clustereventtypes").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched clusterEventType.
func (c *clusterEventTypes) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.ClusterEventType, err error) {
	result = &v1alpha1.ClusterEventType{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("clustereventtypes").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
