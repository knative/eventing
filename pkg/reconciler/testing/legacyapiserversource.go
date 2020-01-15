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
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/apis/legacysources/v1alpha1"
	"knative.dev/eventing/pkg/utils"
)

// LegacyApiServerSourceOption enables further configuration of a Legacy ApiServerSource.
type LegacyApiServerSourceOption func(*v1alpha1.ApiServerSource)

// NewLegacyApiServerSource creates a ApiServer with ApiServerOptions
func NewLegacyApiServerSource(name, namespace string, o ...LegacyApiServerSourceOption) *v1alpha1.ApiServerSource {
	c := &v1alpha1.ApiServerSource{
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

func WithLegacyApiServerSourceUID(uid string) LegacyApiServerSourceOption {
	return func(a *v1alpha1.ApiServerSource) {
		a.UID = types.UID(uid)
	}
}

// WithInitApiServerSourceConditions initializes the ApiServerSource's conditions.
func WithInitLegacyApiServerSourceConditions(s *v1alpha1.ApiServerSource) {
	s.Status.InitializeConditions()
	s.MarkDeprecated(&s.Status.Status, "ApiServerSourceDeprecated", "apiserversources.sources.eventing.knative.dev are deprecated and will be removed in the future. Use apiserversources.sources.knative.dev instead.")
}

func WithLegacyApiServerSourceSinkNotFound(s *v1alpha1.ApiServerSource) {
	s.Status.MarkNoSink("NotFound", "")
}

func WithLegacyApiServerSourceSink(uri string) LegacyApiServerSourceOption {
	return func(s *v1alpha1.ApiServerSource) {
		s.Status.MarkSink(uri)
	}
}

func WithLegacyApiServerSourceSinkDepRef(uri string) LegacyApiServerSourceOption {
	return func(s *v1alpha1.ApiServerSource) {
		s.Status.MarkSinkWarnRefDeprecated(uri)
	}
}

func WithLegacyApiServerSourceDeploymentUnavailable(s *v1alpha1.ApiServerSource) {
	// The Deployment uses GenerateName, so its name is empty.
	name := utils.GenerateFixedName(s, fmt.Sprintf("apiserversource-%s", s.Name))
	s.Status.PropagateDeploymentAvailability(NewDeployment(name, "any"))
}

func WithLegacyApiServerSourceDeployed(s *v1alpha1.ApiServerSource) {
	s.Status.PropagateDeploymentAvailability(NewDeployment("any", "any", WithDeploymentAvailable()))
}

func WithLegacyApiServerSourceEventTypes(s *v1alpha1.ApiServerSource) {
	s.Status.MarkEventTypes()
}

func WithLegacyApiServerSourceSufficientPermissions(s *v1alpha1.ApiServerSource) {
	s.Status.MarkSufficientPermissions()
}

func WithLegacyApiServerSourceNoSufficientPermissions(s *v1alpha1.ApiServerSource) {
	s.Status.MarkNoSufficientPermissions("", `User system:serviceaccount:testnamespace:default cannot get, list, watch resource "namespaces" in API group ""`)
}

func WithLegacyApiServerSourceDeleted(c *v1alpha1.ApiServerSource) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	c.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithLegacyApiServerSourceSpec(spec v1alpha1.ApiServerSourceSpec) LegacyApiServerSourceOption {
	return func(c *v1alpha1.ApiServerSource) {
		c.Spec = spec
	}
}

func WithLegacyApiServerSourceStatusObservedGeneration(generation int64) LegacyApiServerSourceOption {
	return func(c *v1alpha1.ApiServerSource) {
		c.Status.ObservedGeneration = generation
	}
}

func WithLegacyApiServerSourceObjectMetaGeneration(generation int64) LegacyApiServerSourceOption {
	return func(c *v1alpha1.ApiServerSource) {
		c.ObjectMeta.Generation = generation
	}
}
