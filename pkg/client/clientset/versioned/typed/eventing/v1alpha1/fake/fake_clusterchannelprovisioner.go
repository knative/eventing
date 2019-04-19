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
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeClusterChannelProvisioners implements ClusterChannelProvisionerInterface
type FakeClusterChannelProvisioners struct {
	Fake *FakeEventingV1alpha1
}

var clusterchannelprovisionersResource = schema.GroupVersionResource{Group: "eventing.knative.dev", Version: "v1alpha1", Resource: "clusterchannelprovisioners"}

var clusterchannelprovisionersKind = schema.GroupVersionKind{Group: "eventing.knative.dev", Version: "v1alpha1", Kind: "ClusterChannelProvisioner"}

// Get takes name of the clusterChannelProvisioner, and returns the corresponding clusterChannelProvisioner object, and an error if there is any.
func (c *FakeClusterChannelProvisioners) Get(name string, options v1.GetOptions) (result *v1alpha1.ClusterChannelProvisioner, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusterchannelprovisionersResource, name), &v1alpha1.ClusterChannelProvisioner{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ClusterChannelProvisioner), err
}

// List takes label and field selectors, and returns the list of ClusterChannelProvisioners that match those selectors.
func (c *FakeClusterChannelProvisioners) List(opts v1.ListOptions) (result *v1alpha1.ClusterChannelProvisionerList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusterchannelprovisionersResource, clusterchannelprovisionersKind, opts), &v1alpha1.ClusterChannelProvisionerList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.ClusterChannelProvisionerList{ListMeta: obj.(*v1alpha1.ClusterChannelProvisionerList).ListMeta}
	for _, item := range obj.(*v1alpha1.ClusterChannelProvisionerList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterChannelProvisioners.
func (c *FakeClusterChannelProvisioners) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clusterchannelprovisionersResource, opts))
}

// Create takes the representation of a clusterChannelProvisioner and creates it.  Returns the server's representation of the clusterChannelProvisioner, and an error, if there is any.
func (c *FakeClusterChannelProvisioners) Create(clusterChannelProvisioner *v1alpha1.ClusterChannelProvisioner) (result *v1alpha1.ClusterChannelProvisioner, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusterchannelprovisionersResource, clusterChannelProvisioner), &v1alpha1.ClusterChannelProvisioner{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ClusterChannelProvisioner), err
}

// Update takes the representation of a clusterChannelProvisioner and updates it. Returns the server's representation of the clusterChannelProvisioner, and an error, if there is any.
func (c *FakeClusterChannelProvisioners) Update(clusterChannelProvisioner *v1alpha1.ClusterChannelProvisioner) (result *v1alpha1.ClusterChannelProvisioner, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusterchannelprovisionersResource, clusterChannelProvisioner), &v1alpha1.ClusterChannelProvisioner{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ClusterChannelProvisioner), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeClusterChannelProvisioners) UpdateStatus(clusterChannelProvisioner *v1alpha1.ClusterChannelProvisioner) (*v1alpha1.ClusterChannelProvisioner, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(clusterchannelprovisionersResource, "status", clusterChannelProvisioner), &v1alpha1.ClusterChannelProvisioner{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ClusterChannelProvisioner), err
}

// Delete takes name of the clusterChannelProvisioner and deletes it. Returns an error if one occurs.
func (c *FakeClusterChannelProvisioners) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(clusterchannelprovisionersResource, name), &v1alpha1.ClusterChannelProvisioner{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeClusterChannelProvisioners) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusterchannelprovisionersResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.ClusterChannelProvisionerList{})
	return err
}

// Patch applies the patch and returns the patched clusterChannelProvisioner.
func (c *FakeClusterChannelProvisioners) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.ClusterChannelProvisioner, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterchannelprovisionersResource, name, data, subresources...), &v1alpha1.ClusterChannelProvisioner{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ClusterChannelProvisioner), err
}
