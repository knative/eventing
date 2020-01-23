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
	"knative.dev/pkg/apis"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
)

// PingSourceOption enables further configuration of a CronJob.
type PingSourceOption func(*v1alpha1.PingSource)

// NewPingSource creates a PingSource with CronJobOptions.
func NewPingSource(name, namespace string, o ...PingSourceOption) *v1alpha1.PingSource {
	c := &v1alpha1.PingSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range o {
		opt(c)
	}
	// c.SetDefaults(context.Background()) // TODO: We should add defaults and validation.
	return c
}

func WithPingSourceUID(uid string) PingSourceOption {
	return func(c *v1alpha1.PingSource) {
		c.UID = types.UID(uid)
	}
}

// WithInitPingSourceConditions initializes the PingSource's conditions.
func WithInitPingSourceConditions(s *v1alpha1.PingSource) {
	s.Status.InitializeConditions()
}

func WithValidPingSourceSchedule(s *v1alpha1.PingSource) {
	s.Status.MarkSchedule()
}

func WithInvalidPingSourceSchedule(s *v1alpha1.PingSource) {
	s.Status.MarkInvalidSchedule("Invalid", "")
}

func WithPingSourceSinkNotFound(s *v1alpha1.PingSource) {
	s.Status.MarkNoSink("NotFound", "")
}

func WithPingSourceSink(uri *apis.URL) PingSourceOption {
	return func(s *v1alpha1.PingSource) {
		s.Status.MarkSink(uri)
	}
}

func WithPingSourceDeployed(s *v1alpha1.PingSource) {
	s.Status.PropagateDeploymentAvailability(NewDeployment("any", "any", WithDeploymentAvailable()))
}

func WithPingSourceEventType(s *v1alpha1.PingSource) {
	s.Status.MarkEventType()
}

func WithValidPingSourceResources(s *v1alpha1.PingSource) {
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
