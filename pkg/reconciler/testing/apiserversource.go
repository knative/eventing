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
	"knative.dev/eventing/pkg/apis/sources/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
)

// V1Alpha2ApiServerSourceOption enables further configuration of a v1alpha2 ApiServer.
type V1Alpha2ApiServerSourceOption func(*v1alpha2.ApiServerSource)

// V1Beta1ApiServerSourceOption enables further configuration of a v1beta1 ApiServer.
type V1Beta1ApiServerSourceOption func(*v1beta1.ApiServerSource)

// NewApiServerSourceV1Alpha2 creates a v1alpha2 ApiServer with ApiServerOptions
func NewApiServerSourceV1Alpha2(name, namespace string, o ...V1Alpha2ApiServerSourceOption) *v1alpha2.ApiServerSource {
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

// NewApiServerSourceV1Beta1 creates a v1beta1 ApiServer with ApiServerOptions
func NewApiServerSourceV1Beta1(name, namespace string, o ...V1Beta1ApiServerSourceOption) *v1beta1.ApiServerSource {
	c := &v1beta1.ApiServerSource{
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

func WithApiServerSourceUIDV1B1(uid string) V1Beta1ApiServerSourceOption {
	return func(a *v1beta1.ApiServerSource) {
		a.UID = types.UID(uid)
	}
}

// WithInitApiServerSourceConditionsV1B1 initializes the v1beta1 ApiServerSource's conditions.
func WithInitApiServerSourceConditionsV1B1(s *v1beta1.ApiServerSource) {
	s.Status.InitializeConditions()
}

func WithApiServerSourceSinkNotFoundV1B1(s *v1beta1.ApiServerSource) {
	s.Status.MarkNoSink("NotFound", "")
}

func WithApiServerSourceSinkV1B1(uri *apis.URL) V1Beta1ApiServerSourceOption {
	return func(s *v1beta1.ApiServerSource) {
		s.Status.MarkSink(uri)
	}
}

func WithApiServerSourceDeploymentUnavailableV1B1(s *v1beta1.ApiServerSource) {
	// The Deployment uses GenerateName, so its name is empty.
	name := kmeta.ChildName(fmt.Sprintf("apiserversource-%s-", s.Name), string(s.GetUID()))
	s.Status.PropagateDeploymentAvailability(NewDeployment(name, "any"))
}

func WithApiServerSourceDeployedV1B1(s *v1beta1.ApiServerSource) {
	s.Status.PropagateDeploymentAvailability(NewDeployment("any", "any", WithDeploymentAvailable()))
}

func WithApiServerSourceEventTypesV1B1(source string) V1Beta1ApiServerSourceOption {
	return func(s *v1beta1.ApiServerSource) {
		ceAttributes := make([]duckv1.CloudEventAttributes, 0, len(v1beta1.ApiServerSourceEventTypes))
		for _, apiServerSourceType := range v1beta1.ApiServerSourceEventTypes {
			ceAttributes = append(ceAttributes, duckv1.CloudEventAttributes{
				Type:   apiServerSourceType,
				Source: source,
			})
		}
		s.Status.CloudEventAttributes = ceAttributes
	}
}

func WithApiServerSourceSufficientPermissionsV1B1(s *v1beta1.ApiServerSource) {
	s.Status.MarkSufficientPermissions()
}

func WithApiServerSourceNoSufficientPermissionsV1B1(s *v1beta1.ApiServerSource) {
	s.Status.MarkNoSufficientPermissions("", `User system:serviceaccount:testnamespace:default cannot get, list, watch resource "namespaces" in API group ""`)
}

func WithApiServerSourceDeletedV1B1(c *v1beta1.ApiServerSource) {
	t := metav1.NewTime(time.Unix(1e9, 0))
	c.ObjectMeta.SetDeletionTimestamp(&t)
}

func WithApiServerSourceSpecV1A2(spec v1alpha2.ApiServerSourceSpec) V1Alpha2ApiServerSourceOption {
	return func(c *v1alpha2.ApiServerSource) {
		c.Spec = spec
		c.Spec.SetDefaults(context.Background())
	}
}

func WithApiServerSourceSpecV1B1(spec v1beta1.ApiServerSourceSpec) V1Beta1ApiServerSourceOption {
	return func(c *v1beta1.ApiServerSource) {
		c.Spec = spec
		c.Spec.SetDefaults(context.Background())
	}
}

func WithApiServerSourceStatusObservedGenerationV1B1(generation int64) V1Beta1ApiServerSourceOption {
	return func(c *v1beta1.ApiServerSource) {
		c.Status.ObservedGeneration = generation
	}
}

func WithApiServerSourceObjectMetaGenerationV1B1(generation int64) V1Beta1ApiServerSourceOption {
	return func(c *v1beta1.ApiServerSource) {
		c.ObjectMeta.Generation = generation
	}
}
