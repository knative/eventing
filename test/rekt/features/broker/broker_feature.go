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

package broker

import (
	"context"
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"

	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/test/rekt/resources/broker"
)

const (
	interval = 3 * time.Second
	timeout  = 1 * time.Minute
)

func BrokerGoesReady(name string) *feature.Feature {
	gvr := schema.GroupVersionResource{Group: "eventing.knative.dev", Version: "v1", Resource: "brokers"}

	f := new(feature.Feature)

	f.Setup(fmt.Sprintf("install broker %q", name), broker.Install(name))

	f.Stable("broker").
		Must("be ready",
			func(ctx context.Context, t *testing.T) {
				env := environment.FromContext(ctx)
				if err := k8s.WaitForResourceReady(ctx, env.Namespace(), name, gvr, interval, timeout); err != nil {
					t.Error("broker did not go ready, ", err)
				}
			}).
		Must("be addressable", func(ctx context.Context, t *testing.T) {
			env := environment.FromContext(ctx)
			c := eventingclient.Get(ctx)
			err := wait.PollImmediate(1*time.Second, 10*time.Second, func() (bool, error) {
				b, err := c.EventingV1().Brokers(env.Namespace()).Get(ctx, name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				if b.Status.Address.URL == nil {
					return false, fmt.Errorf("broker has no status.address.url, %w", err)
				}
				return true, nil
			})
			if err != nil {
				t.Error("broker has no status.address.url, ", err)
			}
		})

	return f
}
