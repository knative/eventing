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

package sut

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	retryCount    = int32(12)
	backoffPolicy = eventingduckv1.BackoffPolicyExponential
	backoffDelay  = "PT1S"
)

// BrokerAndTriggers will deploy a default broker and 2 triggers to route two types of
// events to receiver.
type BrokerAndTriggers struct {
	Broker
	Triggers
}

// Broker will hold settings for broker itself
type Broker struct {
	Name string
	Opts []resources.BrokerOption
}

// Triggers will hold settings for triggers
type Triggers struct {
	Types []string
}

// NewBrokerAndTriggers will create default configuration for BrokerAndTriggers
// based SUT.
func NewBrokerAndTriggers() SystemUnderTest {
	return &BrokerAndTriggers{
		Broker: Broker{
			Name: "sut",
			Opts: []resources.BrokerOption{
				resources.WithDeliveryForBroker(
					&eventingduckv1.DeliverySpec{
						Retry:         &retryCount,
						BackoffPolicy: &backoffPolicy,
						BackoffDelay:  &backoffDelay,
					}),
			},
		},
		Triggers: Triggers{
			Types: eventTypes,
		},
	}
}

func (b *BrokerAndTriggers) Deploy(ctx Context, dest duckv1.Destination) interface{} {
	b.deployBroker(ctx)
	url := b.fetchURL(ctx)
	b.deployTriggers(ctx, dest)
	return url
}

func (b *BrokerAndTriggers) deployBroker(ctx Context) {
	ctx.Client.CreateBrokerOrFail(b.Name, b.Broker.Opts...)
}

func (b *BrokerAndTriggers) fetchURL(ctx Context) *apis.URL {
	namespace := ctx.Client.Namespace
	ctx.Log.Debugf("Fetching \"%s\" broker URL for ns %s",
		b.Name, namespace)
	meta := resources.NewMetaResource(
		b.Name, namespace, testlib.BrokerTypeMeta,
	)
	err := duck.WaitForResourceReady(ctx.Client.Dynamic, meta)
	if err != nil {
		ctx.T.Fatal(err)
	}
	broker, err := ctx.Client.Eventing.EventingV1().Brokers(namespace).Get(
		ctx.Ctx, b.Name, metav1.GetOptions{},
	)
	if err != nil {
		ctx.T.Fatal(err)
	}
	url := broker.Status.AddressStatus.Address.URL
	ctx.Log.Debugf("\"%s\" broker URL for ns %s is %v",
		b.Name, namespace, url)
	return url
}

func (b *BrokerAndTriggers) deployTriggers(ctx Context, dest duckv1.Destination) {
	for _, eventType := range b.Triggers.Types {
		name := fmt.Sprintf("%s-%s", b.Name, eventType)
		subscriberOption := resources.WithSubscriberDestination(func(t *eventingv1.Trigger) duckv1.Destination {
			return dest
		})
		ctx.Log.Debugf("Creating trigger \"%s\" for type %s to route to %#v",
			name, eventType, dest)
		_ = ctx.Client.CreateTriggerOrFail(
			name,
			resources.WithBroker(b.Name),
			resources.WithAttributesTriggerFilter(
				eventingv1.TriggerAnyFilter,
				eventType,
				map[string]interface{}{},
			),
			subscriberOption,
		)
	}
}
