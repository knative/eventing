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

	legacysourcesv1alpha1 "knative.dev/eventing/pkg/apis/legacysources/v1alpha1"
	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/tracker"
)

// SinkBindingOption enables further configuration of a SinkBinding.
type SinkBindingOption func(*sourcesv1alpha1.SinkBinding)

// LegacySinkBindingOption enables further configuration of a SinkBinding.
type LegacySinkBindingOption func(*legacysourcesv1alpha1.SinkBinding)

// NewLegacySinkBinding creates a SinkBinding with SinkBindingOptions
func NewLegacySinkBinding(name, namespace string, o ...LegacySinkBindingOption) *legacysourcesv1alpha1.SinkBinding {
	c := &legacysourcesv1alpha1.SinkBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range o {
		opt(c)
	}
	//c.SetDefaults(context.Background()) // TODO: We should add defaults and validation.
	return c
}

// NewSinkBinding creates a SinkBinding with SinkBindingOptions
func NewSinkBinding(name, namespace string, o ...SinkBindingOption) *sourcesv1alpha1.SinkBinding {
	c := &sourcesv1alpha1.SinkBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range o {
		opt(c)
	}
	//c.SetDefaults(context.Background()) // TODO: We should add defaults and validation.
	return c
}

// WithLegacySubject assigns the subject of the SinkBinding.
func WithLegacySubject(subject tracker.Reference) LegacySinkBindingOption {
	return func(sb *legacysourcesv1alpha1.SinkBinding) {
		sb.Spec.Subject = subject
	}
}

// WithSubject assigns the subject of the SinkBinding.
func WithSubject(subject tracker.Reference) SinkBindingOption {
	return func(sb *sourcesv1alpha1.SinkBinding) {
		sb.Spec.Subject = subject
	}
}

// WithLegacySink assigns the sink of the SinkBinding.
func WithLegacySink(sink duckv1.Destination) LegacySinkBindingOption {
	return func(sb *legacysourcesv1alpha1.SinkBinding) {
		sb.Spec.Sink = sink
	}
}

// WithSink assigns the sink of the SinkBinding.
func WithSink(sink duckv1.Destination) SinkBindingOption {
	return func(sb *sourcesv1alpha1.SinkBinding) {
		sb.Spec.Sink = sink
	}
}

// WithCloudEventOverrides assigns the CloudEventsOverrides of the SinkBinding.
func WithCloudEventOverrides(overrides duckv1.CloudEventOverrides) SinkBindingOption {
	return func(sb *sourcesv1alpha1.SinkBinding) {
		sb.Spec.CloudEventOverrides = &overrides
	}
}

// ContainerSourceOption enables further configuration of a ContainerSource.
type ContainerSourceOption func(*legacysourcesv1alpha1.ContainerSource)

// NewContainerSource creates a ContainerSource with ContainerSourceOptions
func NewContainerSource(name, namespace string, o ...ContainerSourceOption) *legacysourcesv1alpha1.ContainerSource {
	c := &legacysourcesv1alpha1.ContainerSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range o {
		opt(c)
	}
	//c.SetDefaults(context.Background()) // TODO: We should add defaults and validation.
	return c
}

func WithContainerSourceUID(uid types.UID) ContainerSourceOption {
	return func(s *legacysourcesv1alpha1.ContainerSource) {
		s.UID = uid
	}
}

// WithInitContainerSourceConditions initializes the ContainerSource's conditions.
func WithInitContainerSourceConditions(s *legacysourcesv1alpha1.ContainerSource) {
	s.Status.InitializeConditions()
	s.MarkDeprecated(&s.Status.Status, "ContainerSourceDeprecated", "containersources.sources.eventing.knative.dev are deprecated and will be removed in the future. Use a Deployment and SinkBinding.sources.knative.dev instead.")
}

func WithContainerSourceSinkNotFound(msg string) ContainerSourceOption {
	return func(s *legacysourcesv1alpha1.ContainerSource) {
		s.Status.MarkNoSink("NotFound", msg)
	}
}

func WithContainerSourceSinkMissing(msg string) ContainerSourceOption {
	return func(s *legacysourcesv1alpha1.ContainerSource) {
		s.Status.MarkNoSink("Missing", msg)
	}
}

func WithContainerSourceSink(uri string) ContainerSourceOption {
	return func(s *legacysourcesv1alpha1.ContainerSource) {
		s.Status.MarkSink(uri)
	}
}

func WithContainerSourceDeploying(msg string) ContainerSourceOption {
	return func(s *legacysourcesv1alpha1.ContainerSource) {
		s.Status.MarkDeploying("DeploymentCreated", msg)
	}
}

func WithContainerSourceDeployFailed(msg string) ContainerSourceOption {
	return func(s *legacysourcesv1alpha1.ContainerSource) {
		s.Status.MarkNotDeployed("DeploymentCreateFailed", msg)
	}
}

func WithContainerSourceDeployed(s *legacysourcesv1alpha1.ContainerSource) {
	s.Status.MarkDeployed()
}

func WithContainerSourceDeleted(c *legacysourcesv1alpha1.ContainerSource) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	c.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithContainerSourceSpec(spec legacysourcesv1alpha1.ContainerSourceSpec) ContainerSourceOption {
	return func(c *legacysourcesv1alpha1.ContainerSource) {
		c.Spec = spec
	}
}

func WithContainerSourceLabels(labels map[string]string) ContainerSourceOption {
	return func(c *legacysourcesv1alpha1.ContainerSource) {
		c.Labels = labels
	}
}

func WithContainerSourceAnnotations(annotations map[string]string) ContainerSourceOption {
	return func(c *legacysourcesv1alpha1.ContainerSource) {
		c.Annotations = annotations
	}
}

func WithContainerSourceStatusObservedGeneration(generation int64) ContainerSourceOption {
	return func(c *legacysourcesv1alpha1.ContainerSource) {
		c.Status.ObservedGeneration = generation
	}
}

func WithContainerSourceObjectMetaGeneration(generation int64) ContainerSourceOption {
	return func(c *legacysourcesv1alpha1.ContainerSource) {
		c.ObjectMeta.Generation = generation
	}
}
