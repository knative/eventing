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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
)

// CronJobSourceOption enables further configuration of a CronJob.
type CronJobSourceOption func(*v1alpha1.CronJobSource)

// NewCronJobSource creates a CronJobSource with CronJobOptions.
func NewCronJobSource(name, namespace string, o ...CronJobSourceOption) *v1alpha1.CronJobSource {
	c := &v1alpha1.CronJobSource{
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

func WithCronJobSourceUID(uid string) CronJobSourceOption {
	return func(c *v1alpha1.CronJobSource) {
		c.UID = types.UID(uid)
	}
}

// WithInitCronJobSourceConditions initializes the CronJobSource's conditions.
func WithInitCronJobSourceConditions(s *v1alpha1.CronJobSource) {
	s.Status.InitializeConditions()
}

func WithValidCronJobSourceSchedule(s *v1alpha1.CronJobSource) {
	s.Status.MarkSchedule()
}

func WithInvalidCronJobSourceSchedule(s *v1alpha1.CronJobSource) {
	s.Status.MarkInvalidSchedule("Invalid", "")
}

func WithCronJobSourceSinkNotFound(s *v1alpha1.CronJobSource) {
	s.Status.MarkNoSink("NotFound", "")
}

func WithCronJobSourceSink(uri string) CronJobSourceOption {
	return func(s *v1alpha1.CronJobSource) {
		s.Status.MarkSink(uri)
	}
}

func WithCronJobSourceDeployed(s *v1alpha1.CronJobSource) {
	s.Status.PropagateDeploymentAvailability(NewDeployment("any", "any", WithDeploymentAvailable()))
}

func WithCronJobSourceEventType(s *v1alpha1.CronJobSource) {
	s.Status.MarkEventType()
}

func WithValidCronJobSourceResources(s *v1alpha1.CronJobSource) {
	s.Status.MarkResourcesCorrect()
}

func WithCronJobSourceDeleted(c *v1alpha1.CronJobSource) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	c.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithCronJobSourceSpec(spec v1alpha1.CronJobSourceSpec) CronJobSourceOption {
	return func(c *v1alpha1.CronJobSource) {
		c.Spec = spec
	}
}
