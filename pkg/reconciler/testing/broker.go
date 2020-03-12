/*
Copyright 2019 The Knative Authors

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
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	"knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1alpha1/broker"
	"knative.dev/pkg/apis"
)

// BrokerOption enables further configuration of a Broker.
type BrokerOption func(*v1alpha1.Broker)

// NewBroker creates a Broker with BrokerOptions.
func NewBroker(name, namespace string, o ...BrokerOption) *v1alpha1.Broker {
	b := &v1alpha1.Broker{
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
func WithInitBrokerConditions(b *v1alpha1.Broker) {
	b.Status.InitializeConditions()
}

func WithBrokerFinalizers(finalizers ...string) BrokerOption {
	return func(b *v1alpha1.Broker) {
		b.Finalizers = finalizers
	}
}

func WithBrokerResourceVersion(rv string) BrokerOption {
	return func(b *v1alpha1.Broker) {
		b.ResourceVersion = rv
	}
}

func WithBrokerGeneration(gen int64) BrokerOption {
	return func(s *v1alpha1.Broker) {
		s.Generation = gen
	}
}

func WithBrokerStatusObservedGeneration(gen int64) BrokerOption {
	return func(s *v1alpha1.Broker) {
		s.Status.ObservedGeneration = gen
	}
}

func WithBrokerDeletionTimestamp(b *v1alpha1.Broker) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	b.ObjectMeta.SetDeletionTimestamp(&t)
}

// WithBrokerChannel sets the Broker's ChannelTemplateSpec to the specified CRD.
func WithBrokerChannel(crdType metav1.TypeMeta) BrokerOption {
	return func(b *v1alpha1.Broker) {
		b.Spec.ChannelTemplate = &messagingv1beta1.ChannelTemplateSpec{
			TypeMeta: crdType,
		}
	}
}

// WithBrokerAddress sets the Broker's address.
func WithBrokerAddress(address string) BrokerOption {
	return func(b *v1alpha1.Broker) {
		b.Status.SetAddress(&apis.URL{
			Scheme: "http",
			Host:   address,
		})
	}
}

// WithBrokerReady sets .Status to ready.
func WithBrokerReady(b *v1alpha1.Broker) {
	b.Status = *v1alpha1.TestHelper.ReadyBrokerStatus()
}

// WithTriggerChannelFailed calls .Status.MarkTriggerChannelFailed on the Broker.
func WithTriggerChannelFailed(reason, msg string) BrokerOption {
	return func(b *v1alpha1.Broker) {
		b.Status.MarkTriggerChannelFailed(reason, msg)
	}
}

// WithFilterFailed calls .Status.MarkFilterFailed on the Broker.
func WithFilterFailed(reason, msg string) BrokerOption {
	return func(b *v1alpha1.Broker) {
		b.Status.MarkFilterFailed(reason, msg)
	}
}

// WithIngressFailed calls .Status.MarkIngressFailed on the Broker.
func WithIngressFailed(reason, msg string) BrokerOption {
	return func(b *v1alpha1.Broker) {
		b.Status.MarkIngressFailed(reason, msg)
	}
}

// WithTriggerChannelReady calls .Status.PropagateTriggerChannelReadiness on the Broker.
func WithTriggerChannelReady() BrokerOption {
	return func(b *v1alpha1.Broker) {
		b.Status.PropagateTriggerChannelReadiness(v1alpha1.TestHelper.ReadyChannelStatus())
	}
}

func WithFilterDeploymentAvailable() BrokerOption {
	return func(b *v1alpha1.Broker) {
		b.Status.PropagateFilterDeploymentAvailability(v1alpha1.TestHelper.AvailableDeployment())
	}
}

func WithIngressDeploymentAvailable() BrokerOption {
	return func(b *v1alpha1.Broker) {
		b.Status.PropagateIngressDeploymentAvailability(v1alpha1.TestHelper.AvailableDeployment())
	}
}

func WithBrokerTriggerChannel(c *corev1.ObjectReference) BrokerOption {
	return func(b *v1alpha1.Broker) {
		b.Status.TriggerChannel = c
	}
}

func WithBrokerClass(bc string) BrokerOption {
	return func(b *v1alpha1.Broker) {
		annotations := b.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string, 1)
		}
		annotations[broker.ClassAnnotationKey] = bc
		b.SetAnnotations(annotations)
	}
}
