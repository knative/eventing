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

	"knative.dev/eventing/pkg/apis/eventing"

	"knative.dev/eventing/pkg/apis/sources/v1alpha2"

	"knative.dev/pkg/apis"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
)

// PingSourceOption enables further configuration of a CronJob.
type PingSourceOption func(*v1alpha1.PingSource)

// PingSourceV1A2Option enables further configuration of a CronJob.
type PingSourceV1A2Option func(*v1alpha2.PingSource)

// NewPingSourceV1Alpha1 creates a PingSource with PingSourceOption.
func NewPingSourceV1Alpha1(name, namespace string, o ...PingSourceOption) *v1alpha1.PingSource {
	c := &v1alpha1.PingSource{
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

func WithPingSourceUID(uid string) PingSourceOption {
	return func(c *v1alpha1.PingSource) {
		c.UID = types.UID(uid)
	}
}

func WithPingSourceResourceScopeAnnotation(c *v1alpha1.PingSource) {
	if c.Annotations == nil {
		c.Annotations = make(map[string]string)
	}
	c.Annotations[eventing.ScopeAnnotationKey] = eventing.ScopeResource
}

func WithPingSourceV1A2ResourceScopeAnnotation(c *v1alpha2.PingSource) {
	if c.Annotations == nil {
		c.Annotations = make(map[string]string)
	}
	c.Annotations[eventing.ScopeAnnotationKey] = eventing.ScopeResource
}

func WithPingSourceClusterScopeAnnotation(c *v1alpha1.PingSource) {
	if c.Annotations == nil {
		c.Annotations = make(map[string]string)
	}
	c.Annotations[eventing.ScopeAnnotationKey] = eventing.ScopeCluster
}

func WithPingSourceV1A2ClusterScopeAnnotation(c *v1alpha2.PingSource) {
	if c.Annotations == nil {
		c.Annotations = make(map[string]string)
	}
	c.Annotations[eventing.ScopeAnnotationKey] = eventing.ScopeCluster
}

// WithInitPingSourceConditions initializes the PingSource's conditions.
func WithInitPingSourceConditions(s *v1alpha1.PingSource) {
	s.Status.InitializeConditions()
}

func WithInitPingSourceV1A2Conditions(s *v1alpha2.PingSource) {
	s.Status.InitializeConditions()
}

func WithValidPingSourceSchedule(s *v1alpha1.PingSource) {
	s.Status.MarkSchedule()
}

func WithValidPingSourceV1A2Schedule(s *v1alpha2.PingSource) {
	s.Status.MarkSchedule()
}

func WithInvalidPingSourceSchedule(s *v1alpha1.PingSource) {
	s.Status.MarkInvalidSchedule("Invalid", "")
}

func WithPingSourceSinkNotFound(s *v1alpha1.PingSource) {
	s.Status.MarkNoSink("NotFound", "")
}

func WithPingSourceV1A2SinkNotFound(s *v1alpha2.PingSource) {
	s.Status.MarkNoSink("NotFound", "")
}

func WithPingSourceSink(uri *apis.URL) PingSourceOption {
	return func(s *v1alpha1.PingSource) {
		s.Status.MarkSink(uri)
	}
}

func WithPingSourceV1A2Sink(uri *apis.URL) PingSourceV1A2Option {
	return func(s *v1alpha2.PingSource) {
		s.Status.MarkSink(uri)
	}
}

func WithPingSourceNotDeployed(name string) PingSourceOption {
	return func(s *v1alpha1.PingSource) {
		s.Status.PropagateDeploymentAvailability(NewDeployment(name, "any"))
	}
}

func WithPingSourceDeployed(s *v1alpha1.PingSource) {
	s.Status.PropagateDeploymentAvailability(NewDeployment("any", "any", WithDeploymentAvailable()))
}

func WithPingSourceV1A2Deployed(s *v1alpha2.PingSource) {
	s.Status.PropagateDeploymentAvailability(NewDeployment("any", "any", WithDeploymentAvailable()))
}

func WithPingSourceEventType(s *v1alpha1.PingSource) {
	s.Status.MarkEventType()
}

func WithPingSourceV1A2EventType(s *v1alpha2.PingSource) {
	s.Status.MarkEventType()
}

func WithValidPingSourceResources(s *v1alpha1.PingSource) {
	s.Status.MarkResourcesCorrect()
}

func WithValidPingSourceV1A2Resources(s *v1alpha2.PingSource) {
	s.Status.MarkResourcesCorrect()
}

func WithPingSourceDeleted(c *v1alpha1.PingSource) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	c.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithPingSourceSpec(spec v1alpha1.PingSourceSpec) PingSourceOption {
	return func(c *v1alpha1.PingSource) {
		c.Spec = spec
	}
}

func WithPingSourceV1A2Spec(spec v1alpha2.PingSourceSpec) PingSourceV1A2Option {
	return func(c *v1alpha2.PingSource) {
		c.Spec = spec
	}
}

func WithPingSourceApiVersion(apiVersion string) PingSourceOption {
	return func(c *v1alpha1.PingSource) {
		c.APIVersion = apiVersion
	}
}

func WithPingSourceStatusObservedGeneration(generation int64) PingSourceOption {
	return func(c *v1alpha1.PingSource) {
		c.Status.ObservedGeneration = generation
	}
}

func WithPingSourceObjectMetaGeneration(generation int64) PingSourceOption {
	return func(c *v1alpha1.PingSource) {
		c.ObjectMeta.Generation = generation
	}
}
