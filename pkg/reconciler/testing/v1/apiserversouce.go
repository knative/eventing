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
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apisources "knative.dev/eventing/pkg/apis/sources"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/pkg/reconciler/testing"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
)

// ApiServerSourceOption enables further configuration of a v1 ApiServer.
type ApiServerSourceOption func(*v1.ApiServerSource)

// NewApiServerSource creates a v1 ApiServer with ApiServerOptions
func NewApiServerSource(name, namespace string, o ...ApiServerSourceOption) *v1.ApiServerSource {
	c := &v1.ApiServerSource{
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

func WithApiServerSourceUID(uid string) ApiServerSourceOption {
	return func(a *v1.ApiServerSource) {
		a.UID = types.UID(uid)
	}
}

// WithInitApiServerSourceConditions initializes the v1 ApiServerSource's conditions.
func WithInitApiServerSourceConditions(s *v1.ApiServerSource) {
	s.Status.InitializeConditions()
}

func WithApiServerSourceSinkNotFound(s *v1.ApiServerSource) {
	s.Status.MarkNoSink("NotFound", "")
}

func WithApiServerSourceSink(uri *apis.URL) ApiServerSourceOption {
	return func(s *v1.ApiServerSource) {
		s.Status.MarkSink(uri)
	}
}

func WithApiServerSourceDeploymentUnavailable(s *v1.ApiServerSource) {
	// The Deployment uses GenerateName, so its name is empty.
	name := kmeta.ChildName(fmt.Sprintf("apiserversource-%s-", s.Name), string(s.GetUID()))
	s.Status.PropagateDeploymentAvailability(testing.NewDeployment(name, "any"))
}

func WithApiServerSourceDeployed(s *v1.ApiServerSource) {
	s.Status.PropagateDeploymentAvailability(testing.NewDeployment("any", "any", testing.WithDeploymentAvailable()))
}

func WithApiServerSourceReferenceModeEventTypes(source string) ApiServerSourceOption {
	return func(s *v1.ApiServerSource) {
		ceAttributes := make([]duckv1.CloudEventAttributes, 0, len(apisources.ApiServerSourceEventReferenceModeTypes))
		for _, apiServerSourceType := range apisources.ApiServerSourceEventReferenceModeTypes {
			ceAttributes = append(ceAttributes, duckv1.CloudEventAttributes{
				Type:   apiServerSourceType,
				Source: source,
			})
		}
		s.Status.CloudEventAttributes = ceAttributes
	}
}

func WithApiServerSourceResourceModeEventTypes(source string) ApiServerSourceOption {
	return func(s *v1.ApiServerSource) {
		ceAttributes := make([]duckv1.CloudEventAttributes, 0, len(apisources.ApiServerSourceEventResourceModeTypes))
		for _, apiServerSourceType := range apisources.ApiServerSourceEventResourceModeTypes {
			ceAttributes = append(ceAttributes, duckv1.CloudEventAttributes{
				Type:   apiServerSourceType,
				Source: source,
			})
		}
		s.Status.CloudEventAttributes = ceAttributes
	}
}

func WithApiServerSourceSufficientPermissions(s *v1.ApiServerSource) {
	s.Status.MarkSufficientPermissions()
}

func WithApiServerSourceNoSufficientPermissions(s *v1.ApiServerSource) {
	s.Status.MarkNoSufficientPermissions("", `User system:serviceaccount:testnamespace:default cannot get, list, watch resource "namespaces" in API group ""`)
}

func WithApiServerSourceDeleted(c *v1.ApiServerSource) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	c.ObjectMeta.SetDeletionTimestamp(&t)
}
func WithApiServerSourceSpec(spec v1.ApiServerSourceSpec) ApiServerSourceOption {
	return func(c *v1.ApiServerSource) {
		c.Spec = spec
		c.Spec.SetDefaults(context.Background())
	}
}

func WithApiServerSourceStatusObservedGeneration(generation int64) ApiServerSourceOption {
	return func(c *v1.ApiServerSource) {
		c.Status.ObservedGeneration = generation
	}
}

func WithApiServerSourceObjectMetaGeneration(generation int64) ApiServerSourceOption {
	return func(c *v1.ApiServerSource) {
		c.ObjectMeta.Generation = generation
	}
}
