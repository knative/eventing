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

package test

import (
	"log"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// IsRevisionReady will check the status conditions of the revision and return true if the revision is
// ready to serve traffic. It will return false if the status indicates a state other than deploying
// or being ready. It will also return false if the type of the condition is unexpected.
func IsRevisionReady(r *servingv1alpha1.Revision) (bool, error) {
	return r.Status.IsReady(), nil
}

// IsServiceReady will check the status conditions of the service and return true if the service is
// ready. This means that its configurations and routes have all reported ready.
func IsServiceReady(s *servingv1alpha1.Service) (bool, error) {
	return s.Status.IsReady(), nil
}

// IsRouteReady will check the status conditions of the route and return true if the route is
// ready.
func IsRouteReady(r *servingv1alpha1.Route) (bool, error) {
	return r.Status.IsReady(), nil
}

// IsChannelReady will check the status conditions of the Channel and return true
// if the Channel is ready.
func IsChannelReady(c *eventingv1alpha1.Channel) (bool, error) {
	return c.Status.IsReady(), nil
}

// IsSubscriptionReady will check the status conditions of the Subscription and
// return true if the Subscription is ready.
func IsSubscriptionReady(s *eventingv1alpha1.Subscription) (bool, error) {
	return s.Status.IsReady(), nil
}

// PodsRunning will check the status conditions of the pod list and return true
// if all pods are Running.
func PodsRunning(podList *corev1.PodList) (bool, error) {
	var names []string
	for _, p := range podList.Items {
		names = append(names, p.Name)
	}
	log.Printf("Checking pods: %v", names)
	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodSucceeded {
			return false, nil
		}
	}
	return true, nil
}
