/*
Copyright 2021 The Knative Authors

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

package trigger

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	eventingclientsetv1 "knative.dev/eventing/pkg/client/clientset/versioned/typed/eventing/v1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/state"
)

const (
	TriggerNameKey = "triggerName"
)

func GetTrigger(ctx context.Context, t feature.T) *eventingv1.Trigger {
	c := Client(ctx)
	name := state.GetStringOrFail(ctx, t, TriggerNameKey)

	trigger, err := c.Triggers.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		t.Errorf("failed to get Trigger, %v", err)
	}
	return trigger
}

func SetTriggerName(name string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		state.SetOrFail(ctx, t, TriggerNameKey, name)
	}
}

type EventingClient struct {
	Brokers  eventingclientsetv1.BrokerInterface
	Triggers eventingclientsetv1.TriggerInterface
}

func Client(ctx context.Context) *EventingClient {
	ec := eventingclient.Get(ctx).EventingV1()
	env := environment.FromContext(ctx)

	return &EventingClient{
		Brokers:  ec.Brokers(env.Namespace()),
		Triggers: ec.Triggers(env.Namespace()),
	}
}
