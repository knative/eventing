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

package base

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
	// The interval and timeout used for polling in checking resource states.
	interval = 1 * time.Second
	timeout  = 4 * time.Minute
)

// WaitForChannelState polls the status of the Channel called name from client
// every interval until inState returns `true` indicating it is done, returns an
// error or timeout. desc will be used to name the metric that is emitted to
// track how long it took for name to get into the state checked by inState.
func WaitForChannelState(client eventingclient.ChannelInterface, name string, inState func(c *eventingv1alpha1.Channel) (bool, error), desc string) error {
	metricName := fmt.Sprintf("WaitForChannelState/%s/%s", name, desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		c, err := client.Get(name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return inState(c)
	})
}

// WaitForChannelListState polls the status of the ChannelList
// from client every interval until inState returns `true` indicating it
// is done, returns an error or timeout. desc will be used to name the metric
// that is emitted to track how long it took to get into the state checked by inState.
func WaitForChannelListState(client eventingclient.ChannelInterface, inState func(cl *eventingv1alpha1.ChannelList) (bool, error), desc string) error {
	metricName := fmt.Sprintf("WaitForChannelListState/%s", desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		cl, err := client.List(metav1.ListOptions{})
		if err != nil {
			return true, err
		}
		return inState(cl)
	})
}

// WaitForSubscriptionState polls the status of the Subscription called name
// from client every interval until inState returns `true` indicating it is
// done, returns an error or timeout. desc will be used to name the metric that
// is emitted to track how long it took for name to get into the state checked
// by inState.
func WaitForSubscriptionState(client eventingclient.SubscriptionInterface, name string, inState func(s *eventingv1alpha1.Subscription) (bool, error), desc string) error {
	metricName := fmt.Sprintf("WaitForSubscriptionState/%s/%s", name, desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		s, err := client.Get(name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return inState(s)
	})
}

// WaitForSubscriptionListState polls the status of the SubscriptionList
// from client every interval until inState returns `true` indicating it
// is done, returns an error or timeout. desc will be used to name the metric
// that is emitted to track how long it took to get into the state checked by inState.
func WaitForSubscriptionListState(client eventingclient.SubscriptionInterface, inState func(sl *eventingv1alpha1.SubscriptionList) (bool, error), desc string) error {
	metricName := fmt.Sprintf("WaitForSubscriptionListState/%s", desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		sl, err := client.List(metav1.ListOptions{})
		if err != nil {
			return true, err
		}
		return inState(sl)
	})
}

// WaitForBrokerState polls the status of the Broker called name from client
// every interval until inState returns `true` indicating it is done, returns an
// error or timeout. desc will be used to name the metric that is emitted to
// track how long it took for name to get into the state checked by inState.
func WaitForBrokerState(client eventingclient.BrokerInterface, name string, inState func(b *eventingv1alpha1.Broker) (bool, error), desc string) error {
	metricName := fmt.Sprintf("WaitForBrokerState/%s/%s", name, desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		b, err := client.Get(name, metav1.GetOptions{})
		if k8serrors.IsNotFound(err) {
			// Return false as we are not done yet.
			// We swallow the error to keep on polling
			return false, nil
		} else if err != nil {
			// Return true to stop and return the error.
			return true, err
		}
		return inState(b)
	})
}

// WaitForBrokerListState polls the status of the BrokerList
// from client every interval until inState returns `true` indicating it
// is done, returns an error or timeout. desc will be used to name the metric
// that is emitted to track how long it took to get into the state checked by inState.
func WaitForBrokerListState(client eventingclient.BrokerInterface, inState func(bl *eventingv1alpha1.BrokerList) (bool, error), desc string) error {
	metricName := fmt.Sprintf("WaitForBrokerListState/%s", desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		bl, err := client.List(metav1.ListOptions{})
		if err != nil {
			return true, err
		}
		return inState(bl)
	})
}

// WaitForTriggerState polls the status of the Trigger called name from client
// every interval until inState returns `true` indicating it is done, returns an
// error or timeout. desc will be used to name the metric that is emitted to
// track how long it took for name to get into the state checked by inState.
func WaitForTriggerState(client eventingclient.TriggerInterface, name string, inState func(t *eventingv1alpha1.Trigger) (bool, error), desc string) error {
	metricName := fmt.Sprintf("WaitForTriggerState/%s/%s", name, desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		t, err := client.Get(name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return inState(t)
	})
}

// WaitForTriggerListState polls the status of the TriggerList
// from client every interval until inState returns `true` indicating it
// is done, returns an error or timeout. desc will be used to name the metric
// that is emitted to track how long it took to get into the state checked by inState.
func WaitForTriggerListState(client eventingclient.TriggerInterface, inState func(tl *eventingv1alpha1.TriggerList) (bool, error), desc string) error {
	metricName := fmt.Sprintf("WaitForTriggerListState/%s", desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		tl, err := client.List(metav1.ListOptions{})
		if err != nil {
			return true, err
		}
		return inState(tl)
	})
}
