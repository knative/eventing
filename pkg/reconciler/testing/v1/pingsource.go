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

package testing

import (
	"context"
	"time"

	"knative.dev/eventing/pkg/reconciler/testing"

	duckv1 "knative.dev/pkg/apis/duck/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "knative.dev/eventing/pkg/apis/sources/v1"
)

// PingSourceOption enables further configuration of a CronJob.
type PingSourceOption func(*v1.PingSource)

// NewPingSource creates a PingSource with PingSourceOption.
func NewPingSource(name, namespace string, o ...PingSourceOption) *v1.PingSource {
	c := &v1.PingSource{
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

func WithPingSource(uid string) PingSourceOption {
	return func(c *v1.PingSource) {
		c.UID = types.UID(uid)
	}
}

func WithInitPingSourceConditions(s *v1.PingSource) {
	s.Status.InitializeConditions()
}

func WithPingSourceSinkNotFound(s *v1.PingSource) {
	s.Status.MarkNoSink("NotFound", "")
}

func WithPingSourceSink(addr *duckv1.Addressable) PingSourceOption {
	return func(s *v1.PingSource) {
		s.Status.MarkSink(addr)
	}
}

func WithPingSourceDeployed(s *v1.PingSource) {
	s.Status.PropagateDeploymentAvailability(testing.NewDeployment("any", "any", testing.WithDeploymentAvailable()))
}

func WithPingSourceCloudEventAttributes(s *v1.PingSource) {
	s.Status.CloudEventAttributes = []duckv1.CloudEventAttributes{{
		Type:   v1.PingSourceEventType,
		Source: v1.PingSourceSource(s.Namespace, s.Name),
	}}
}

func WithPingSourceSpec(spec v1.PingSourceSpec) PingSourceOption {
	return func(c *v1.PingSource) {
		c.Spec = spec
	}
}

func WithPingSourceStatusObservedGeneration(generation int64) PingSourceOption {
	return func(c *v1.PingSource) {
		c.Status.ObservedGeneration = generation
	}
}

func WithPingSourceObjectMetaGeneration(generation int64) PingSourceOption {
	return func(c *v1.PingSource) {
		c.ObjectMeta.Generation = generation
	}
}

func WithPingSourceFinalizers(finalizers ...string) PingSourceOption {
	return func(c *v1.PingSource) {
		c.Finalizers = finalizers
	}
}

func WithPingSourceDeleted(c *v1.PingSource) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	c.SetDeletionTimestamp(&t)
}
