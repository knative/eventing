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
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	servingclient "github.com/knative/serving/pkg/client/clientset/versioned/typed/serving/v1alpha1"
	"go.opencensus.io/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	interval = 1 * time.Second
	timeout  = 1 * time.Minute
)

// WaitForRouteState polls the status of the Route called name from client every
// interval until inState returns `true` indicating it is done, returns an
// error or timeout. desc will be used to name the metric that is emitted to
// track how long it took for name to get into the state checked by inState.
func WaitForRouteState(client servingclient.RouteInterface, name string, inState func(r *servingv1alpha1.Route) (bool, error), desc string) error {
	metricName := fmt.Sprintf("WaitForRouteState/%s/%s", name, desc)
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
