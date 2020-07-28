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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	b.Status = *v1.TestHelper.ReadyBrokerStatus()
}

func WithDeprecatedStatus(b *v1.Broker) {
	dc := apis.Condition{
		Type:     "Deprecated",
		Reason:   "SingleTenantChannelBrokerDeprecated",
		Status:   corev1.ConditionTrue,
		Severity: apis.ConditionSeverityWarning,
		Message:  "Single Tenant Channel Brokers are deprecated and will be removed in release 0.16. Use Multi Tenant Channel Brokers instead.",
	}

	for i, c := range b.Status.Status.Conditions {
		if c.Type == dc.Type {
			b.Status.Status.Conditions[i] = dc
			return
		}
	}
	b.Status.Status.Conditions = append(b.Status.Status.Conditions, dc)
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
