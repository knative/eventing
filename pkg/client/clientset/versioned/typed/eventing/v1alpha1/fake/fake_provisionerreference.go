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

package fake

import (
	v1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeProvisionerReferences implements ProvisionerReferenceInterface
type FakeProvisionerReferences struct {
	Fake *FakeEventingV1alpha1
	ns   string
}

var provisionerreferencesResource = schema.GroupVersionResource{Group: "eventing.knative.dev", Version: "v1alpha1", Resource: "provisionerreferences"}

var provisionerreferencesKind = schema.GroupVersionKind{Group: "eventing.knative.dev", Version: "v1alpha1", Kind: "ProvisionerReference"}

// Get takes name of the provisionerReference, and returns the corresponding provisionerReference object, and an error if there is any.
func (c *FakeProvisionerReferences) Get(name string, options v1.GetOptions) (result *v1alpha1.ProvisionerReference, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(provisionerreferencesResource, c.ns, name), &v1alpha1.ProvisionerReference{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ProvisionerReference), err
}

// List takes label and field selectors, and returns the list of ProvisionerReferences that match those selectors.
func (c *FakeProvisionerReferences) List(opts v1.ListOptions) (result *v1alpha1.ProvisionerReferenceList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(provisionerreferencesResource, provisionerreferencesKind, c.ns, opts), &v1alpha1.ProvisionerReferenceList{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ProvisionerReferenceList), err
}

// Watch returns a watch.Interface that watches the requested provisionerReferences.
func (c *FakeProvisionerReferences) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(provisionerreferencesResource, c.ns, opts))

}

// Create takes the representation of a provisionerReference and creates it.  Returns the server's representation of the provisionerReference, and an error, if there is any.
func (c *FakeProvisionerReferences) Create(provisionerReference *v1alpha1.ProvisionerReference) (result *v1alpha1.ProvisionerReference, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(provisionerreferencesResource, c.ns, provisionerReference), &v1alpha1.ProvisionerReference{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ProvisionerReference), err
}

// Update takes the representation of a provisionerReference and updates it. Returns the server's representation of the provisionerReference, and an error, if there is any.
func (c *FakeProvisionerReferences) Update(provisionerReference *v1alpha1.ProvisionerReference) (result *v1alpha1.ProvisionerReference, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(provisionerreferencesResource, c.ns, provisionerReference), &v1alpha1.ProvisionerReference{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ProvisionerReference), err
}

// Delete takes name of the provisionerReference and deletes it. Returns an error if one occurs.
func (c *FakeProvisionerReferences) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(provisionerreferencesResource, c.ns, name), &v1alpha1.ProvisionerReference{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeProvisionerReferences) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(provisionerreferencesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.ProvisionerReferenceList{})
	return err
}

// Patch applies the patch and returns the patched provisionerReference.
func (c *FakeProvisionerReferences) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.ProvisionerReference, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(provisionerreferencesResource, c.ns, name, data, subresources...), &v1alpha1.ProvisionerReference{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ProvisionerReference), err
}
