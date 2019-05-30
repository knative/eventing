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

	duckv1alpha1 "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/pkg/apis"
)

// ChannelOption enables further configuration of a Channel.
type ChannelOption func(*v1alpha1.Channel)

// NewChannel creates a Channel with ChannelOptions
func NewChannel(name, namespace string, o ...ChannelOption) *v1alpha1.Channel {
	c := &v1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range o {
		opt(c)
	}
	c.SetDefaults(context.Background())
	return c
}

// NewChannelWithoutNamespace creates a Channel with ChannelOptions but without a specific namespace
func NewChannelWithoutNamespace(name string, o ...ChannelOption) *v1alpha1.Channel {
	s := &v1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	for _, opt := range o {
		opt(s)
	}
	s.SetDefaults(context.Background())
	return s
}

// WithInitChannelConditions initializes the Channel's conditions.
func WithInitChannelConditions(s *v1alpha1.Channel) {
	s.Status.InitializeConditions()
}

func WithChannelDeleted(c *v1alpha1.Channel) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	c.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithChannelProvisionerNotFound(name, kind string) ChannelOption {
	return func(c *v1alpha1.Channel) {
		c.Status.MarkProvisionerNotInstalled(
			"Provisioner not found.",
			"Specified provisioner [Name:%s Kind:%s] is not installed or not controlling the channel.",
			name,
			kind,
		)
	}
}

func WithChannelProvisioner(gvk metav1.GroupVersionKind, name string) ChannelOption {
	return func(c *v1alpha1.Channel) {
		c.Spec.Provisioner = &corev1.ObjectReference{
			APIVersion: apiVersion(gvk),
			Kind:       gvk.Kind,
			Name:       name,
		}
	}
}

func WithChannelAddress(hostname string) ChannelOption {
	return func(c *v1alpha1.Channel) {
		c.Status.SetAddress(&apis.URL{
			Scheme: "http",
			Host:   hostname,
		})
	}
}

func WithChannelReady(c *v1alpha1.Channel) {
	c.Status = *v1alpha1.TestHelper.ReadyChannelStatus()
}

func WithChannelSubscribers(subscribers []duckv1alpha1.ChannelSubscriberSpec) ChannelOption {
	return func(c *v1alpha1.Channel) {
		c.Spec.Subscribable = &duckv1alpha1.Subscribable{
			Subscribers: subscribers,
		}
	}
}

func WithChannelGenerateName(generateName string) ChannelOption {
	return func(c *v1alpha1.Channel) {
		c.ObjectMeta.GenerateName = generateName
	}
}

func WithChannelLabels(labels map[string]string) ChannelOption {
	return func(c *v1alpha1.Channel) {
		c.ObjectMeta.Labels = labels
	}
}

func WithChannelOwnerReferences(ownerReferences []metav1.OwnerReference) ChannelOption {
	return func(c *v1alpha1.Channel) {
		c.ObjectMeta.OwnerReferences = ownerReferences
	}
}
