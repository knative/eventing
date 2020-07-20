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

	sourcesv1alpha2 "knative.dev/eventing/pkg/apis/sources/v1alpha2"

	sourcesv1beta1 "knative.dev/eventing/pkg/apis/sources/v1beta1"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// ContainerSourceV1Alpha2Option enables further configuration of a ContainerSource.
type ContainerSourceV1Alpha2Option func(*sourcesv1alpha2.ContainerSource)

// ContainerSourceV1Beta1Option enables further configuration of a ContainerSource.
type ContainerSourceV1Beta1Option func(*sourcesv1beta1.ContainerSource)

// NewContainerSource creates a v1alpha2 ContainerSource with ContainerSourceOptions
func NewContainerSourceV1Alpha2(name, namespace string, o ...ContainerSourceV1Alpha2Option) *sourcesv1alpha2.ContainerSource {
	c := &sourcesv1alpha2.ContainerSource{
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

// NewContainerSource creates a v1beta1 ContainerSource with ContainerSourceOptions
func NewContainerSourceV1Beta1(name, namespace string, o ...ContainerSourceV1Beta1Option) *sourcesv1beta1.ContainerSource {
	c := &sourcesv1beta1.ContainerSource{
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

func WithContainerSourceUIDV1B1(uid types.UID) ContainerSourceV1Beta1Option {
	return func(s *sourcesv1beta1.ContainerSource) {
		s.UID = uid
	}
}

// WithInitContainerSourceConditions initializes the ContainerSource's conditions.
func WithInitContainerSourceConditionsV1B1(s *sourcesv1beta1.ContainerSource) {
	s.Status.InitializeConditions()
}

func WithContainerSourcePropagateReceiveAdapterStatusV1B1(d *appsv1.Deployment) ContainerSourceV1Beta1Option {
	return func(s *sourcesv1beta1.ContainerSource) {
		s.Status.PropagateReceiveAdapterStatus(d)
	}
}

func WithContainerSourcePropagateSinkbindingStatusV1B1(status *sourcesv1beta1.SinkBindingStatus) ContainerSourceV1Beta1Option {
	return func(s *sourcesv1beta1.ContainerSource) {
		s.Status.PropagateSinkBindingStatus(status)
	}
}

func WithContainerSourceDeleted(c *sourcesv1beta1.ContainerSource) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	c.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithContainerSourceSpecV1A2(spec sourcesv1alpha2.ContainerSourceSpec) ContainerSourceV1Alpha2Option {
	return func(c *sourcesv1alpha2.ContainerSource) {
		c.Spec = spec
	}
}

func WithContainerSourceSpecV1B1(spec sourcesv1beta1.ContainerSourceSpec) ContainerSourceV1Beta1Option {
	return func(c *sourcesv1beta1.ContainerSource) {
		c.Spec = spec
	}
}

func WithContainerSourceLabelsV1B1(labels map[string]string) ContainerSourceV1Beta1Option {
	return func(c *sourcesv1beta1.ContainerSource) {
		c.Labels = labels
	}
}

func WithContainerSourceAnnotationsV1B1(annotations map[string]string) ContainerSourceV1Beta1Option {
	return func(c *sourcesv1beta1.ContainerSource) {
		c.Annotations = annotations
	}
}

func WithContainerSourceStatusObservedGenerationV1B1(generation int64) ContainerSourceV1Beta1Option {
	return func(c *sourcesv1beta1.ContainerSource) {
		c.Status.ObservedGeneration = generation
	}
}

func WithContainerSourceObjectMetaGenerationV1B1(generation int64) ContainerSourceV1Beta1Option {
	return func(c *sourcesv1beta1.ContainerSource) {
		c.ObjectMeta.Generation = generation
	}
}

func WithContainerUnobservedGenerationV1B1() ContainerSourceV1Beta1Option {
	return func(c *sourcesv1beta1.ContainerSource) {
		condSet := c.GetConditionSet()
		condSet.Manage(&c.Status).MarkUnknown(
			condSet.GetTopLevelConditionType(), "NewObservedGenFailure", "unsuccessfully observed a new generation")
	}
}
