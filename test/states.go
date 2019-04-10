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
	corev1 "k8s.io/api/core/v1"
)

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

// IsBrokerReady will check the status conditions of the Broker and return true
// if the Broker is ready.
func IsBrokerReady(b *eventingv1alpha1.Broker) (bool, error) {
	return b.Status.IsReady(), nil
}

// IsTriggerReady will check the status conditions of the Trigger and
// return true if the Trigger is ready.
func IsTriggerReady(t *eventingv1alpha1.Trigger) (bool, error) {
	return t.Status.IsReady(), nil
}

// TriggersReady will check the status conditions of the trigger list and return true
// if all triggers are Ready.
func TriggersReady(triggerList *eventingv1alpha1.TriggerList) (bool, error) {
	var names []string
	for _, t := range triggerList.Items {
		names = append(names, t.Name)
	}
	log.Printf("Checking triggers: %v", names)
	for _, trigger := range triggerList.Items {
		if !trigger.Status.IsReady() {
			return false, nil
		}
	}
	return true, nil
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
