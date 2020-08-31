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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
	"knative.dev/eventing/pkg/apis/sources/v1beta1"
)

// PingSourceV1A2Option enables further configuration of a CronJob.
type PingSourceV1A2Option func(*v1alpha2.PingSource)

// PingSourceV1B1Option enables further configuration of a CronJob.
type PingSourceV1B1Option func(*v1beta1.PingSource)

// NewPingSourceV1Alpha2 creates a PingSource with PingSourceOption.
func NewPingSourceV1Alpha2(name, namespace string, o ...PingSourceV1A2Option) *v1alpha2.PingSource {
	c := &v1alpha2.PingSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range o {
		opt(c)
	}
	c.SetDefaults(context.Background()) // TODO: We should add defaults and validation.
	return c
}

// NewPingSourceV1Beta1 creates a PingSource with PingSourceOption.
func NewPingSourceV1Beta1(name, namespace string, o ...PingSourceV1B1Option) *v1beta1.PingSource {
	c := &v1beta1.PingSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range o {
		opt(c)
	}
	c.SetDefaults(context.Background()) // TODO: We should add defaults and validation.
	return c
}

func WithPingSourceV1A2UID(uid string) PingSourceV1A2Option {
	return func(c *v1alpha2.PingSource) {
		c.UID = types.UID(uid)
	}
}

func WithInitPingSourceV1A2Conditions(s *v1alpha2.PingSource) {
	s.Status.InitializeConditions()
}

func WithValidPingSourceV1A2Schedule(s *v1alpha2.PingSource) {
	s.Status.MarkSchedule()
}

func WithPingSourceV1A2SinkNotFound(s *v1alpha2.PingSource) {
	s.Status.MarkNoSink("NotFound", "")
}

func WithPingSourceV1A2Sink(uri *apis.URL) PingSourceV1A2Option {
	return func(s *v1alpha2.PingSource) {
		s.Status.MarkSink(uri)
	}
}

func WithPingSourceV1A2NotDeployed(name string) PingSourceV1A2Option {
	return func(s *v1alpha2.PingSource) {
		s.Status.PropagateDeploymentAvailability(NewDeployment(name, "any"))
	}
}

func WithPingSourceV1A2Deployed(s *v1alpha2.PingSource) {
	s.Status.PropagateDeploymentAvailability(NewDeployment("any", "any", WithDeploymentAvailable()))
}

func WithPingSourceV1A2CloudEventAttributes(s *v1alpha2.PingSource) {
	s.Status.CloudEventAttributes = []duckv1.CloudEventAttributes{{
		Type:   v1alpha2.PingSourceEventType,
		Source: v1alpha2.PingSourceSource(s.Namespace, s.Name),
	}}
}

func WithValidPingSourceV1A2Resources(s *v1alpha2.PingSource) {
	s.Status.MarkResourcesCorrect()
}

func WithPingSourceV1A2Spec(spec v1alpha2.PingSourceSpec) PingSourceV1A2Option {
	return func(c *v1alpha2.PingSource) {
		c.Spec = spec
	}
}

func WithPingSourceV1A2StatusObservedGeneration(generation int64) PingSourceV1A2Option {
	return func(c *v1alpha2.PingSource) {
		c.Status.ObservedGeneration = generation
	}
}

func WithPingSourceV1A2ObjectMetaGeneration(generation int64) PingSourceV1A2Option {
	return func(c *v1alpha2.PingSource) {
		c.ObjectMeta.Generation = generation
	}
}

func WithPingSourceV1A2Finalizers(finalizers ...string) PingSourceV1A2Option {
	return func(c *v1alpha2.PingSource) {
		c.Finalizers = finalizers
	}
}

func WithPingSourceV1A2Deleted(c *v1alpha2.PingSource) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	c.SetDeletionTimestamp(&t)
}

func WithPingSourceV1B1UID(uid string) PingSourceV1B1Option {
	return func(c *v1beta1.PingSource) {
		c.UID = types.UID(uid)
	}
}

func WithInitPingSourceV1B1Conditions(s *v1beta1.PingSource) {
	s.Status.InitializeConditions()
}

func WithPingSourceV1B1SinkNotFound(s *v1beta1.PingSource) {
	s.Status.MarkNoSink("NotFound", "")
}

func WithPingSourceV1B1Sink(uri *apis.URL) PingSourceV1B1Option {
	return func(s *v1beta1.PingSource) {
		s.Status.MarkSink(uri)
	}
}

func WithPingSourceV1B1NotDeployed(name string) PingSourceV1B1Option {
	return func(s *v1beta1.PingSource) {
		s.Status.PropagateDeploymentAvailability(NewDeployment(name, "any"))
	}
}

func WithPingSourceV1B1Deployed(s *v1beta1.PingSource) {
	s.Status.PropagateDeploymentAvailability(NewDeployment("any", "any", WithDeploymentAvailable()))
}

func WithPingSourceV1B1CloudEventAttributes(s *v1beta1.PingSource) {
	s.Status.CloudEventAttributes = []duckv1.CloudEventAttributes{{
		Type:   v1beta1.PingSourceEventType,
		Source: v1beta1.PingSourceSource(s.Namespace, s.Name),
	}}
}

func WithPingSourceV1B1Spec(spec v1beta1.PingSourceSpec) PingSourceV1B1Option {
	return func(c *v1beta1.PingSource) {
		c.Spec = spec
	}
}

func WithPingSourceV1B1StatusObservedGeneration(generation int64) PingSourceV1B1Option {
	return func(c *v1beta1.PingSource) {
		c.Status.ObservedGeneration = generation
	}
}

func WithPingSourceV1B1ObjectMetaGeneration(generation int64) PingSourceV1B1Option {
	return func(c *v1beta1.PingSource) {
		c.ObjectMeta.Generation = generation
	}
}

func WithPingSourceV1B1Finalizers(finalizers ...string) PingSourceV1B1Option {
	return func(c *v1beta1.PingSource) {
		c.Finalizers = finalizers
	}
}

func WithPingSourceV1B1Deleted(c *v1beta1.PingSource) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	c.SetDeletionTimestamp(&t)
}
