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

	eventingduckv1alpha1 "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func WithBrokerDeletionTimestamp(b *v1alpha1.Broker) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	b.ObjectMeta.SetDeletionTimestamp(&t)
}

// WithBrokerChannelProvisioner sets the Broker's ChannelTemplate provisioner.
func WithBrokerChannelProvisioner(provisioner *corev1.ObjectReference) BrokerOption {
	return func(b *v1alpha1.Broker) {
		b.Spec.DeprecatedChannelTemplate = &v1alpha1.ChannelSpec{
			Provisioner: provisioner,
		}
	}
}

// WithBrokerChannelCRD sets the Broker's ChannelTemplateSpec to the specified CRD.
func WithBrokerChannelCRD(crdType metav1.TypeMeta) BrokerOption {
	return func(b *v1alpha1.Broker) {
		b.Spec.ChannelTemplate = &eventingduckv1alpha1.ChannelTemplateSpec{
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

// WithBrokerReadyDeprecated sets .Status to ready and sets the Deprecated field.
func WithBrokerReadyDeprecated(b *v1alpha1.Broker) {
	b.Status = *v1alpha1.TestHelper.ReadyBrokerStatusDeprecated()
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

// WithIngressChannelFailed calls .Status.MarkIngressChannelFailed on the Broker.
func WithIngressChannelFailed(reason, msg string) BrokerOption {
	return func(b *v1alpha1.Broker) {
		b.Status.MarkIngressChannelFailed(reason, msg)
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

func WithBrokerIngressChannelReady() BrokerOption {
	return func(b *v1alpha1.Broker) {
		b.Status.PropagateIngressChannelReadiness(v1alpha1.TestHelper.ReadyChannelStatus())
	}
}

func WithBrokerDeprecated() BrokerOption {
	return func(b *v1alpha1.Broker) {
		b.Status.MarkDeprecated("ClusterChannelProvisionerDeprecated", "Provisioners are deprecated and will be removed in 0.9. Recommended replacement is CRD based channels using spec.channelTemplateSpec.")
	}
}

func WithBrokerIngressSubscriptionFailed(reason, msg string) BrokerOption {
	return func(b *v1alpha1.Broker) {
		b.Status.MarkIngressSubscriptionFailed(reason, msg)
	}
}

func WithBrokerTriggerChannel(c *corev1.ObjectReference) BrokerOption {
	return func(b *v1alpha1.Broker) {
		b.Status.TriggerChannel = c
	}
}

func WithBrokerIngressChannel(c *corev1.ObjectReference) BrokerOption {
	return func(b *v1alpha1.Broker) {
		b.Status.IngressChannel = c
	}
}
