/*
Copyright 2020 The Knative Authors

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

package resources

// This file contains functions that construct Serving resources.

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgTest "knative.dev/pkg/test"
)

// ServingClient holds clients required to get serving resources
type ServingClient struct {
	Kube    *pkgTest.KubeClient
	Dynamic dynamic.Interface
}

// KServiceRoute represents ksvc route, so how much traffic is routed to given deployment
type KServiceRoute struct {
	TrafficPercent uint8
	DeploymentName string
}

// WithSubscriberKServiceRefForTrigger returns an option that adds a Subscriber
// Knative Service Ref for the given Trigger.
func WithSubscriberKServiceRefForTrigger(name string) TriggerOptionV1Beta1 {
	return func(t *eventingv1beta1.Trigger) {
		if name != "" {
			t.Spec.Subscriber = duckv1.Destination{
				Ref: KnativeRefForKservice(name, t.Namespace),
			}
		}
	}
}

// KnativeRefForKservice return a duck reference for Knative Service
func KnativeRefForKservice(name, namespace string) *duckv1.KReference {
	return &duckv1.KReference{
		Kind:       KServiceKind,
		APIVersion: ServingAPIVersion,
		Name:       name,
		Namespace:  namespace,
	}
}

// KServiceRef returns a Knative Service ObjectReference for a given Service name.
func KServiceRef(name string) *corev1.ObjectReference {
	return pkgTest.CoreV1ObjectReference(KServiceKind, ServingAPIVersion, name)
}

// KServiceRoutes gets routes of given ksvc.
// If ksvc isn't ready yet second return value will be false.
func KServiceRoutes(client ServingClient, name, namespace string) ([]KServiceRoute, bool, error) {
	serving := client.Dynamic.Resource(KServicesGVR).Namespace(namespace)
	unstruct, err := serving.Get(context.Background(), name, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		// Return false as we are not done yet.
		// We swallow the error to keep on polling.
		// It should only happen if we wait for the auto-created resources, like default Broker.
		return nil, false, nil
	} else if err != nil {
		// Return error to stop the polling.
		return nil, false, err
	}

	routes, ready := ksvcRoutes(unstruct)
	return routes, ready, nil
}

// KServiceDeploymentName returns a name of deployment of Knative Service that
// receives 100% of traffic.
// If ksvc isn't ready yet second return value will be false.
func KServiceDeploymentName(client ServingClient, name, namespace string) (string, bool, error) {
	routes, ready, err := KServiceRoutes(client, name, namespace)
	if ready {
		if len(routes) > 1 {
			return "", false, fmt.Errorf("traffic shouldn't be split to more then 1 revision: %v", routes)
		}
		r := routes[0]
		return r.DeploymentName, true, nil
	}

	return "", ready, err
}

func ksvcRoutes(un *unstructured.Unstructured) ([]KServiceRoute, bool) {
	routes := make([]KServiceRoute, 0)
	content := un.UnstructuredContent()
	maybeStatus, ok := content["status"]
	if !ok {
		return routes, false
	}
	status := maybeStatus.(map[string]interface{})
	maybeTraffic, ok := status["traffic"]
	if !ok {
		return routes, false
	}
	traffic := maybeTraffic.([]interface{})
	if len(traffic) == 0 {
		// continue to wait
		return routes, false
	}
	for _, uRoute := range traffic {
		route := uRoute.(map[string]interface{})
		revisionName := route["revisionName"].(string)
		percent := uint8(route["percent"].(int64))
		deploymentName := fmt.Sprintf("%s-deployment", revisionName)
		routes = append(routes, KServiceRoute{
			TrafficPercent: percent,
			DeploymentName: deploymentName,
		})
	}
	return routes, true
}
