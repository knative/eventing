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

package test

import (
	"os"
	"os/signal"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"

	"github.com/knative/pkg/test/logging"
)

// Cleaner holds resources that will be cleaned after test
type Cleaner struct {
	resourcesToClean []ResourceDeleter
	logger           *logging.BaseLogger
	dynamicClient    dynamic.Interface
}

// ResourceDeleter holds the cleaner and name of resource to be cleaned
type ResourceDeleter struct {
	Resource dynamic.ResourceInterface
	Name     string
}

// NewCleaner creates a new Cleaner
func NewCleaner(log *logging.BaseLogger, client dynamic.Interface) *Cleaner {
	cleaner := &Cleaner{
		resourcesToClean: make([]ResourceDeleter, 0, 10),
		logger:           log,
		dynamicClient:    client,
	}
	return cleaner
}

// Add will register a resource to be cleaned by the Clean function
// This function is generic enough so as to be able to register any resources
// Each resource is identified by:
// * group (e.g. serving.knative.dev)
// * version (e.g. v1alpha1)
// * resource's plural (e.g. routes)
// * namespace (use "" if the resource is not tied to any namespace)
// * actual name of the resource (e.g. myroute)
func (c *Cleaner) Add(group string, version string, resource string, namespace string, name string) error {
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}
	var unstructured dynamic.ResourceInterface
	if namespace != "" {
		unstructured = c.dynamicClient.Resource(gvr).Namespace(namespace)
	} else {
		unstructured = c.dynamicClient.Resource(gvr)
	}
	res := ResourceDeleter{
		Resource: unstructured,
		Name:     name,
	}
	//this is actually a prepend, we want to delete resources in reverse order
	c.resourcesToClean = append([]ResourceDeleter{res}, c.resourcesToClean...)
	return nil
}

// Clean will delete all registered resources
func (c *Cleaner) Clean(awaitDeletion bool) error {
	for _, deleter := range c.resourcesToClean {
		r, err := deleter.Resource.Get(deleter.Name, metav1.GetOptions{})
		if err != nil {
			c.logger.Errorf("Failed to get to-be cleaned resource %q : %s", deleter.Name, err)
		} else {
			c.logger.Infof("Cleaning resource: %q\n%+v", deleter.Name, r)
		}
		if err := deleter.Resource.Delete(deleter.Name, nil); err != nil {
			c.logger.Errorf("Error: %v", err)
		} else if awaitDeletion {
			c.logger.Debugf("Waiting for %s to be deleted", deleter.Name)
			if err := wait.PollImmediate(interval, timeout, func() (bool, error) {
				if _, err := deleter.Resource.Get(deleter.Name, metav1.GetOptions{}); err != nil {
					return true, nil
				}
				return false, nil
			}); err != nil {
				c.logger.Errorf("Error: %v", err)
			}
		}
	}
	return nil
}

// CleanupOnInterrupt will execute the function cleanup if an interrupt signal is caught
func CleanupOnInterrupt(cleanup func(), logger *logging.BaseLogger) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			logger.Infof("Test interrupted, cleaning up.")
			cleanup()
			os.Exit(1)
		}
	}()
}
