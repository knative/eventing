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

package testing

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/eventing"
	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/broker"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// BrokerOption enables further configuration of a Broker.
type BrokerOption func(*v1.Broker)

// NewBroker creates a Broker with BrokerOptions.
func NewBroker(name, namespace string, o ...BrokerOption) *v1.Broker {
	b := &v1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	for _, opt := range o {
		opt(b)
	}
	b.SetDefaults(context.Background())
	return b
}

// WithInitBrokerConditions initializes the Broker's conditions.
func WithInitBrokerConditions(b *v1.Broker) {
	b.Status.InitializeConditions()
}

func WithBrokerFinalizers(finalizers ...string) BrokerOption {
	return func(b *v1.Broker) {
		b.Finalizers = finalizers
	}
}

func WithBrokerResourceVersion(rv string) BrokerOption {
	return func(b *v1.Broker) {
		b.ResourceVersion = rv
	}
}

func WithBrokerGeneration(gen int64) BrokerOption {
	return func(s *v1.Broker) {
		s.Generation = gen
	}
}

func WithBrokerStatusObservedGeneration(gen int64) BrokerOption {
	return func(s *v1.Broker) {
		s.Status.ObservedGeneration = gen
	}
}

func WithBrokerDeletionTimestamp(b *v1.Broker) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	b.ObjectMeta.SetDeletionTimestamp(&t)
}

// WithBrokerChannel sets the Broker's ChannelTemplateSpec to the specified CRD.
func WithBrokerConfig(config *duckv1.KReference) BrokerOption {
	return func(b *v1.Broker) {
		b.Spec.Config = config
	}
}

// WithBrokerAddress sets the Broker's address.
func WithBrokerAddress(address string) BrokerOption {
	return func(b *v1.Broker) {
		b.Status.SetAddress(&apis.URL{
			Scheme: "http",
			Host:   address,
		})
	}
}

// WithBrokerAddressURI sets the Broker's address as URI.
func WithBrokerAddressURI(uri *apis.URL) BrokerOption {
	return func(b *v1.Broker) {
		b.Status.SetAddress(uri)
	}
}

// WithBrokerReady sets .Status to ready.
func WithBrokerReady(b *v1.Broker) {
	b.Status = *v1.TestHelper.ReadyBrokerStatusWithoutDLS()
}

// WithBrokerReady sets .Status to ready with the DLS defined.
func WithBrokerReadyWithDLS(b *v1.Broker) {
	b.Status = *v1.TestHelper.ReadyBrokerStatus()
}

// WithTriggerChannelFailed calls .Status.MarkTriggerChannelFailed on the Broker.
func WithTriggerChannelFailed(reason, msg string) BrokerOption {
	return func(b *v1.Broker) {
		b.Status.MarkTriggerChannelFailed(reason, msg)
	}
}

// WithFilterFailed calls .Status.MarkFilterFailed on the Broker.
func WithFilterFailed(reason, msg string) BrokerOption {
	return func(b *v1.Broker) {
		b.Status.MarkFilterFailed(reason, msg)
	}
}

// WithIngressFailed calls .Status.MarkIngressFailed on the Broker.
func WithIngressFailed(reason, msg string) BrokerOption {
	return func(b *v1.Broker) {
		b.Status.MarkIngressFailed(reason, msg)
	}
}

// WithTriggerChannelReady calls .Status.PropagateTriggerChannelReadiness on the Broker.
func WithTriggerChannelReady() BrokerOption {
	return func(b *v1.Broker) {
		b.Status.PropagateTriggerChannelReadiness(v1.TestHelper.ReadyChannelStatus())
	}
}

func WithFilterAvailable() BrokerOption {
	return func(b *v1.Broker) {
		b.Status.PropagateFilterAvailability(v1.TestHelper.AvailableEndpoints())
	}
}

func WithIngressAvailable() BrokerOption {
	return func(b *v1.Broker) {
		b.Status.PropagateIngressAvailability(v1.TestHelper.AvailableEndpoints())
	}
}

func WithBrokerClass(bc string) BrokerOption {
	return func(b *v1.Broker) {
		annotations := b.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string, 1)
		}
		annotations[broker.ClassAnnotationKey] = bc
		b.SetAnnotations(annotations)
	}
}

func WithChannelAddressAnnotation(address string) BrokerOption {
	return func(b *v1.Broker) {
		if b.Status.Annotations == nil {
			b.Status.Annotations = make(map[string]string, 1)
		}
		b.Status.Annotations[eventing.BrokerChannelAddressStatusAnnotationKey] = address
	}
}

func WithBrokerStatusDLSURI(dlsURI *apis.URL) BrokerOption {
	return func(b *v1.Broker) {
		b.Status.MarkDeadLetterSinkResolvedSucceeded(dlsURI)
	}
}

func WithChannelAPIVersionAnnotation(apiVersion string) BrokerOption {
	return func(b *v1.Broker) {
		if b.Status.Annotations == nil {
			b.Status.Annotations = make(map[string]string, 1)
		}
		b.Status.Annotations[eventing.BrokerChannelAPIVersionStatusAnnotationKey] = apiVersion
	}
}

func WithChannelKindAnnotation(kind string) BrokerOption {
	return func(b *v1.Broker) {
		if b.Status.Annotations == nil {
			b.Status.Annotations = make(map[string]string, 1)
		}
		b.Status.Annotations[eventing.BrokerChannelKindStatusAnnotationKey] = kind
	}
}

func WithChannelNameAnnotation(name string) BrokerOption {
	return func(b *v1.Broker) {
		if b.Status.Annotations == nil {
			b.Status.Annotations = make(map[string]string, 1)
		}
		b.Status.Annotations[eventing.BrokerChannelNameStatusAnnotationKey] = name
	}
}

func WithDeadLeaderSink(ref *duckv1.KReference, uri string) BrokerOption {
	return func(b *v1.Broker) {
		if b.Spec.Delivery == nil {
			b.Spec.Delivery = new(eventingv1.DeliverySpec)
		}
		var u *apis.URL
		if uri != "" {
			u, _ = apis.ParseURL(uri)
		}
		b.Spec.Delivery.DeadLetterSink = &duckv1.Destination{
			Ref: ref,
			URI: u,
		}
	}
}

func WithAddressableUnknown() BrokerOption {
	return func(b *v1.Broker) {
		b.Status.MarkBrokerAddressableUnknown("", "")
	}
}

func WithDLSResolvedFailed() BrokerOption {
	return func(b *v1.Broker) {
		b.Status.MarkDeadLetterSinkResolvedFailed(
			"Unable to get the DeadLetterSink's URI",
			fmt.Sprintf(`brokers.eventing.knative.dev "%s" not found`,
				b.Spec.Delivery.DeadLetterSink.Ref.Name,
			),
		)
	}
}

func WithDLSNotConfigured() BrokerOption {
	return func(b *v1.Broker) {
		b.Status.MarkDeadLetterSinkNotConfigured()
	}
}
