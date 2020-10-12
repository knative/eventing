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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	sourcesv1beta1 "knative.dev/eventing/pkg/apis/sources/v1beta1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/tracker"
)

// SinkBindingOption enables further configuration of a v1beta1 SinkBinding.
type SinkBindingOption func(*sourcesv1beta1.SinkBinding)

// NewSinkBinding creates a SinkBinding with SinkBindingOptions
func NewSinkBinding(name, namespace string, o ...SinkBindingOption) *sourcesv1beta1.SinkBinding {
	c := &sourcesv1beta1.SinkBinding{
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

// WithSubject assigns the subject of the SinkBinding.
func WithSubject(subject tracker.Reference) SinkBindingOption {
	return func(sb *sourcesv1beta1.SinkBinding) {
		sb.Spec.Subject = subject
	}
}

// WithSink assigns the sink of the SinkBinding.
func WithSink(sink duckv1.Destination) SinkBindingOption {
	return func(sb *sourcesv1beta1.SinkBinding) {
		sb.Spec.Sink = sink
	}
}

// WithCloudEventOverrides assigns the CloudEventsOverrides of the SinkBinding.
func WithCloudEventOverrides(overrides duckv1.CloudEventOverrides) SinkBindingOption {
	return func(sb *sourcesv1beta1.SinkBinding) {
		sb.Spec.CloudEventOverrides = &overrides
	}
}
