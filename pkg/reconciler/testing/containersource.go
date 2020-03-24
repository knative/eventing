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

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"

	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
)

// ContainerSourceOption enables further configuration of a ContainerSource.
type ContainerSourceOption func(*sourcesv1alpha1.ContainerSource)

// NewContainerSource creates a ContainerSource with ContainerSourceOptions
func NewContainerSource(name, namespace string, o ...ContainerSourceOption) *sourcesv1alpha1.ContainerSource {
	c := &sourcesv1alpha1.ContainerSource{
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

func WithContainerSourceUID(uid types.UID) ContainerSourceOption {
	return func(s *sourcesv1alpha1.ContainerSource) {
		s.UID = uid
	}
}

// WithInitContainerSourceConditions initializes the ContainerSource's conditions.
func WithInitContainerSourceConditions(s *sourcesv1alpha1.ContainerSource) {
	s.Status.InitializeConditions()
}

func WithContainerSourceSinkNotFound(msg string) ContainerSourceOption {
	return func(s *sourcesv1alpha1.ContainerSource) {
		s.Status.MarkNoSink("SinkNotFound", msg)
	}
}

func WithContainerSourceSinkMissing(msg string) ContainerSourceOption {
	return func(s *sourcesv1alpha1.ContainerSource) {
		s.Status.MarkNoSink("Missing", msg)
	}
}

func WithContainerSourceSink(uri *apis.URL) ContainerSourceOption {
	return func(s *sourcesv1alpha1.ContainerSource) {
		s.Status.MarkSink(uri)
	}
}

func WithContainerSourcePropagateDeploymentAvailability(d *appsv1.Deployment) ContainerSourceOption {
	return func(s *sourcesv1alpha1.ContainerSource) {
		s.Status.PropagateDeploymentAvailability(d)
	}
}

func WithContainerSourceDeployFailed(msg string) ContainerSourceOption {
	return func(s *sourcesv1alpha1.ContainerSource) {
		s.Status.MarkNotDeployed("DeploymentReconcileFailed", msg)
	}
}

func WithContainerSourceDeleted(c *sourcesv1alpha1.ContainerSource) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	c.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithContainerSourceSpec(spec sourcesv1alpha1.ContainerSourceSpec) ContainerSourceOption {
	return func(c *sourcesv1alpha1.ContainerSource) {
		c.Spec = spec
	}
}

func WithContainerSourceLabels(labels map[string]string) ContainerSourceOption {
	return func(c *sourcesv1alpha1.ContainerSource) {
		c.Labels = labels
	}
}

func WithContainerSourceAnnotations(annotations map[string]string) ContainerSourceOption {
	return func(c *sourcesv1alpha1.ContainerSource) {
		c.Annotations = annotations
	}
}

func WithContainerSourceStatusObservedGeneration(generation int64) ContainerSourceOption {
	return func(c *sourcesv1alpha1.ContainerSource) {
		c.Status.ObservedGeneration = generation
	}
}

func WithContainerSourceObjectMetaGeneration(generation int64) ContainerSourceOption {
	return func(c *sourcesv1alpha1.ContainerSource) {
		c.ObjectMeta.Generation = generation
	}
}
