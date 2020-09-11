/*
Copyright 2019 The Knative Authors

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

package lib

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgTest "knative.dev/pkg/test"

	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

// LabelNamespace labels the given namespace with the labels map.
func (c *Client) LabelNamespace(labels map[string]string) error {
	namespace := c.Namespace
	nsSpec, err := c.Kube.Kube.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if nsSpec.Labels == nil {
		nsSpec.Labels = map[string]string{}
	}
	for k, v := range labels {
		nsSpec.Labels[k] = v
	}
	_, err = c.Kube.Kube.CoreV1().Namespaces().Update(context.Background(), nsSpec, metav1.UpdateOptions{})
	return err
}

// GetAddressableURI returns the URI of the addressable resource.
// To use this function, the given resource must have implemented the Addressable duck-type.
func (c *Client) GetAddressableURI(addressableName string, typeMeta *metav1.TypeMeta) (string, error) {
	namespace := c.Namespace
	metaAddressable := resources.NewMetaResource(addressableName, namespace, typeMeta)
	u, err := duck.GetAddressableURI(c.Dynamic, metaAddressable)
	if err != nil {
		return "", errors.WithStack(err)
	}
	return u.String(), nil
}

// WaitForResourceReadyOrFail waits for the resource to become ready or fail.
// To use this function, the given resource must have implemented the Status duck-type.
func (c *Client) WaitForResourceReadyOrFail(name string, typemeta *metav1.TypeMeta) {
	namespace := c.Namespace
	metaResource := resources.NewMetaResource(name, namespace, typemeta)
	if err := duck.WaitForResourceReady(c.Dynamic, metaResource); err != nil {
		untyped, err := duck.GetGenericObject(c.Dynamic, metaResource, &duckv1beta1.KResource{})
		if err != nil {
			c.T.Errorf("Failed to get the object %v-%s when dumping error state: %v", *typemeta, name, err)
		}
		if untyped != nil {
			c.T.Errorf("Object that did not become ready %v-%s when dumping error state: %+v", *typemeta, name, untyped)
		}
		c.T.Fatalf("Failed to get %v-%s ready: %+v", *typemeta, name, errors.WithStack(err))
	}
}

// WaitForResourcesReadyOrFail waits for resources of the given type in the namespace to become ready or fail.
// To use this function, the given resource must have implemented the Status duck-type.
func (c *Client) WaitForResourcesReadyOrFail(typemeta *metav1.TypeMeta) {
	namespace := c.Namespace
	metaResourceList := resources.NewMetaResourceList(namespace, typemeta)
	if err := duck.WaitForResourcesReady(c.Dynamic, metaResourceList); err != nil {
		c.T.Fatalf("Failed to get all %v resources ready: %+v", *typemeta, errors.WithStack(err))
	}
}

// WaitForAllTestResourcesReady waits until all test resources in the namespace are Ready.
func (c *Client) WaitForAllTestResourcesReady(ctx context.Context) error {
	// wait for all Knative resources created in this test to become ready.
	err := c.RetryWebhookErrors(func(attempts int) (err error) {
		e := c.Tracker.WaitForKResourcesReady()
		if e != nil {
			c.T.Logf("Failed to get all KResources ready %v", e)
		}
		return e
	})
	if err != nil {
		return err
	}
	// Explicitly wait for all pods that were created directly by this test to become ready.
	for _, n := range c.podsCreated {
		if err := pkgTest.WaitForPodRunning(ctx, c.Kube, n, c.Namespace); err != nil {
			return fmt.Errorf("created Pod %q did not become ready: %+v", n, errors.WithStack(err))
		}
	}
	// FIXME(chizhg): This hacky sleep is added to try mitigating the test flakiness.
	// Will delete it after we find the root cause and fix.
	time.Sleep(10 * time.Second)
	return nil
}

func (c *Client) WaitForAllTestResourcesReadyOrFail(ctx context.Context) {
	if err := c.WaitForAllTestResourcesReady(ctx); err != nil {
		c.T.Fatalf("Failed to get all test resources ready: %+v", errors.WithStack(err))
	}
}

func (c *Client) WaitForServiceEndpointsOrFail(ctx context.Context, svcName string, numberOfExpectedEndpoints int) {
	c.T.Logf("Waiting for %d endpoints in service %s", numberOfExpectedEndpoints, svcName)
	if err := pkgTest.WaitForServiceEndpoints(ctx, c.Kube, svcName, c.Namespace, numberOfExpectedEndpoints); err != nil {
		c.T.Fatalf("Failed while waiting for %d endpoints in service %s: %+v", numberOfExpectedEndpoints, svcName, errors.WithStack(err))
	}
}
