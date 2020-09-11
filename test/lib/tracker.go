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

// cleanup allows you to define a cleanup function that will be executed
// if your test is interrupted.

package lib

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"

	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"

	"knative.dev/pkg/kmeta"
)

// Tracker holds resources that need to be tracked during test execution.
// It includes:
// 1. KResources that need to check their Ready status;
// 2. All Kubernetes resources that need to be cleaned after test is done.
type Tracker struct {
	resourcesToCheckStatus []resources.MetaResource
	resourcesToClean       []ResourceDeleter
	tc                     *testing.T
	dynamicClient          dynamic.Interface
}

// ResourceDeleter holds the resource interface and name of resource to be cleaned
type ResourceDeleter struct {
	Resource dynamic.ResourceInterface
	Name     string
}

// NewTracker creates a new Tracker
func NewTracker(t *testing.T, client dynamic.Interface) *Tracker {
	tracker := &Tracker{
		resourcesToCheckStatus: make([]resources.MetaResource, 0),
		resourcesToClean:       make([]ResourceDeleter, 0),
		tc:                     t,
		dynamicClient:          client,
	}
	return tracker
}

// Add will register a resource to be cleaned by the Clean function
// This function is generic enough so as to be able to register any resources
// Each resource is identified by:
// * group (e.g. serving.knative.dev)
// * version (e.g. v1alpha1)
// * resource's plural (e.g. routes)
// * namespace (use "" if the resource is not tied to any namespace)
// * actual name of the resource (e.g. myroute)
func (t *Tracker) Add(group string, version string, resource string, namespace string, name string) {
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}
	var unstructured dynamic.ResourceInterface
	if namespace != "" {
		unstructured = t.dynamicClient.Resource(gvr).Namespace(namespace)
	} else {
		unstructured = t.dynamicClient.Resource(gvr)
	}
	res := ResourceDeleter{
		Resource: unstructured,
		Name:     name,
	}
	// this is actually a prepend, we want to delete resources in reverse order
	t.resourcesToClean = append([]ResourceDeleter{res}, t.resourcesToClean...)
}

// AddObj will register a resource that implements OwnerRefable interface to be cleaned by the Clean function.
// It also register the resource for checking if its status is Ready.
// Note this function assumes all resources that implement kmeta.OwnerRefable are KResources.
func (t *Tracker) AddObj(obj kmeta.OwnerRefable) {
	// get the resource's name, namespace and gvr
	name := obj.GetObjectMeta().GetName()
	namespace := obj.GetObjectMeta().GetNamespace()
	gvk := obj.GetGroupVersionKind()
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	metaResource := resources.NewMetaResource(
		name,
		namespace,
		&metav1.TypeMeta{
			Kind:       gvk.Kind,
			APIVersion: gvk.GroupVersion().String(),
		},
	)
	// add the metaResource to the list for future status check
	t.resourcesToCheckStatus = append(t.resourcesToCheckStatus, *metaResource)
	t.Add(gvr.Group, gvr.Version, gvr.Resource, namespace, name)
}

// Clean will delete all registered resources
func (t *Tracker) Clean(awaitDeletion bool) error {
	logf := t.tc.Logf
	for _, deleter := range t.resourcesToClean {
		r, err := deleter.Resource.Get(context.Background(), deleter.Name, metav1.GetOptions{})
		if err != nil {
			logf("Failed to get to-be cleaned resource %q : %v", deleter.Name, err)
		} else {
			// Only log the detailed resource data if the test fails.
			if t.tc.Failed() {
				bytes, _ := json.MarshalIndent(r, "", "  ")
				logf("Cleaning resource: %q\n%+v", deleter.Name, string(bytes))
			} else {
				logf("Cleaning resource: %q", deleter.Name)
			}
		}
		if err := deleter.Resource.Delete(context.Background(), deleter.Name, metav1.DeleteOptions{}); err != nil {
			logf("Failed to clean the resource %q : %v", deleter.Name, err)
		} else if awaitDeletion {
			logf("Waiting for %s to be deleted", deleter.Name)
			if err := wait.PollImmediate(interval, timeout, func() (bool, error) {
				if _, err := deleter.Resource.Get(context.Background(), deleter.Name, metav1.GetOptions{}); err != nil {
					return true, nil
				}
				return false, nil
			}); err != nil {
				logf("Failed to clean the resource %q : %v", deleter.Name, err)
			}
		}
	}
	return nil
}

// WaitForKResourcesReady will wait for all registered KResources to become ready.
func (t *Tracker) WaitForKResourcesReady() error {
	t.tc.Logf("Waiting for all KResources to become ready")
	for _, metaResource := range t.resourcesToCheckStatus {
		if err := duck.WaitForResourceReady(t.dynamicClient, &metaResource); err != nil {
			return fmt.Errorf("failed waiting for %+v to become ready: %v", metaResource, err)
		}
	}
	return nil
}
