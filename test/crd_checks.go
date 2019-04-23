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

// crdpolling contains functions which poll Knative Serving CRDs until they
// get into the state desired by the caller or time out.

package test

import (
	"context"
	"fmt"
	"time"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventingclient "github.com/knative/eventing/pkg/client/clientset/versioned/typed/eventing/v1alpha1"
	"go.opencensus.io/trace"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	interval = 1 * time.Second
	timeout  = 4 * time.Minute
)

// WaitForChannelState polls the status of the Channel called name from client
// every interval until inState returns `true` indicating it is done, returns an
// error or timeout. desc will be used to name the metric that is emitted to
// track how long it took for name to get into the state checked by inState.
func WaitForChannelState(client eventingclient.ChannelInterface, name string, inState func(r *eventingv1alpha1.Channel) (bool, error), desc string) error {
	metricName := fmt.Sprintf("WaitForChannelState/%s/%s", name, desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		r, err := client.Get(name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return inState(r)
	})
}

// WaitForSubscriptionState polls the status of the Subscription called name
// from client every interval until inState returns `true` indicating it is
// done, returns an error or timeout. desc will be used to name the metric that
// is emitted to track how long it took for name to get into the state checked
// by inState.
func WaitForSubscriptionState(client eventingclient.SubscriptionInterface, name string, inState func(r *eventingv1alpha1.Subscription) (bool, error), desc string) error {
	metricName := fmt.Sprintf("WaitForSubscriptionState/%s/%s", name, desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		r, err := client.Get(name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return inState(r)
	})
}

// WaitForBrokerState polls the status of the Broker called name from client
// every interval until inState returns `true` indicating it is done, returns an
// error or timeout. desc will be used to name the metric that is emitted to
// track how long it took for name to get into the state checked by inState.
func WaitForBrokerState(client eventingclient.BrokerInterface, name string, inState func(r *eventingv1alpha1.Broker) (bool, error), desc string) error {
	metricName := fmt.Sprintf("WaitForBrokerState/%s/%s", name, desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		r, err := client.Get(name, metav1.GetOptions{})
		if k8serrors.IsNotFound(err) {
			// Return false as we are not done yet.
			// We swallow the error to keep on polling
			return false, nil
		} else if err != nil {
			// Return true to stop and return the error.
			return true, err
		}
		return inState(r)
	})
}

// WaitForTriggerState polls the status of the Trigger called name from client
// every interval until inState returns `true` indicating it is done, returns an
// error or timeout. desc will be used to name the metric that is emitted to
// track how long it took for name to get into the state checked by inState.
func WaitForTriggerState(client eventingclient.TriggerInterface, name string, inState func(r *eventingv1alpha1.Trigger) (bool, error), desc string) error {
	metricName := fmt.Sprintf("WaitForTriggerState/%s/%s", name, desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		r, err := client.Get(name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return inState(r)
	})
}

// WaitForTriggersListState polls the status of the TriggerList
// from client every interval until inState returns `true` indicating it
// is done, returns an error or timeout. desc will be used to name the metric
// that is emitted to track how long it took to get into the state checked by inState.
func WaitForTriggersListState(clients eventingclient.TriggerInterface, inState func(t *eventingv1alpha1.TriggerList) (bool, error), desc string) error {
	metricName := fmt.Sprintf("WaitForTriggerListState/%s", desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		t, err := clients.List(metav1.ListOptions{})
		if err != nil {
			return true, err
		}
		return inState(t)
	})
}
