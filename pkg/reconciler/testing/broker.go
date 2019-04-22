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

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BrokerOption enables further configuration of a Broker.
type BrokerOption func(*v1alpha1.Broker)

func NewBroker(name, namespace string, o ...BrokerOption) *v1alpha1.Broker {
	b := &v1alpha1.Broker{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "eventing.knative.dev/v1alpha1",
			Kind:       "Broker",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: v1alpha1.BrokerSpec{
			ChannelTemplate: &v1alpha1.ChannelSpec{},
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
		b.Spec.ChannelTemplate.Provisioner = provisioner
	}
}

// MarkTriggerChannelFailed calls .Status.MarkTriggerChannelFailed on the Broker.
func MarkTriggerChannelFailed(reason, format, arg string) BrokerOption {
	return func(b *v1alpha1.Broker) {
		b.Status.MarkTriggerChannelFailed(reason, format, arg)
	}
}
