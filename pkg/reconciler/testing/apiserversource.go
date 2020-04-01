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
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
)

// ApiServerSourceOption enables further configuration of a ApiServer.
type ApiServerSourceOption func(*v1alpha2.ApiServerSource)

// NewApiServerSource creates a ApiServer with ApiServerOptions
func NewApiServerSource(name, namespace string, o ...ApiServerSourceOption) *v1alpha2.ApiServerSource {
	c := &v1alpha2.ApiServerSource{
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
	return func(a *v1alpha2.ApiServerSource) {
		a.UID = types.UID(uid)
	}
}

// WithInitApiServerSourceConditions initializes the ApiServerSource's conditions.
func WithInitApiServerSourceConditions(s *v1alpha2.ApiServerSource) {
	s.Status.InitializeConditions()
}

func WithApiServerSourceSinkNotFound(s *v1alpha2.ApiServerSource) {
	s.Status.MarkNoSink("NotFound", "")
}

func WithApiServerSourceSink(uri *apis.URL) ApiServerSourceOption {
	return func(s *v1alpha2.ApiServerSource) {
		s.Status.MarkSink(uri)
	}
}

func WithApiServerSourceDeploymentUnavailable(s *v1alpha2.ApiServerSource) {
	// The Deployment uses GenerateName, so its name is empty.
	name := kmeta.ChildName(fmt.Sprintf("apiserversource-%s-", s.Name), string(s.GetUID()))
	s.Status.PropagateDeploymentAvailability(NewDeployment(name, "any"))
}

func WithApiServerSourceDeployed(s *v1alpha2.ApiServerSource) {
	s.Status.PropagateDeploymentAvailability(NewDeployment("any", "any", WithDeploymentAvailable()))
}

func WithApiServerSourceEventTypes(source string) ApiServerSourceOption {
	return func(s *v1alpha2.ApiServerSource) {
		ceAttributes := make([]duckv1.CloudEventAttributes, 0, len(v1alpha2.ApiServerSourceEventTypes))
		for _, apiServerSourceType := range v1alpha2.ApiServerSourceEventTypes {
			ceAttributes = append(ceAttributes, duckv1.CloudEventAttributes{
				Type:   apiServerSourceType,
				Source: source,
			})
		}
		s.Status.CloudEventAttributes = ceAttributes
	}
}

func WithApiServerSourceSufficientPermissions(s *v1alpha2.ApiServerSource) {
	s.Status.MarkSufficientPermissions()
}

func WithApiServerSourceNoSufficientPermissions(s *v1alpha2.ApiServerSource) {
	s.Status.MarkNoSufficientPermissions("", `User system:serviceaccount:testnamespace:default cannot get, list, watch resource "namespaces" in API group ""`)
}

func WithApiServerSourceDeleted(c *v1alpha2.ApiServerSource) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	c.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithApiServerSourceSpec(spec v1alpha2.ApiServerSourceSpec) ApiServerSourceOption {
	return func(c *v1alpha2.ApiServerSource) {
		c.Spec = spec
		c.Spec.SetDefaults(context.Background())
	}
}

func WithApiServerSourceStatusObservedGeneration(generation int64) ApiServerSourceOption {
	return func(c *v1alpha2.ApiServerSource) {
		c.Status.ObservedGeneration = generation
	}
}

func WithApiServerSourceObjectMetaGeneration(generation int64) ApiServerSourceOption {
	return func(c *v1alpha2.ApiServerSource) {
		c.ObjectMeta.Generation = generation
	}
}
