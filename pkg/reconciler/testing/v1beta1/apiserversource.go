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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/sources/v1beta1"
)

// ApiServerSourceOption enables further configuration of a v1beta1 ApiServer.
type ApiServerSourceOption func(*v1beta1.ApiServerSource)

// NewApiServerSource creates a v1beta1 ApiServer with ApiServerOptions
func NewApiServerSource(name, namespace string, o ...ApiServerSourceOption) *v1beta1.ApiServerSource {
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

func WithApiServerSourceSpec(spec v1beta1.ApiServerSourceSpec) ApiServerSourceOption {
	return func(c *v1beta1.ApiServerSource) {
		c.Spec = spec
		c.Spec.SetDefaults(context.Background())
	}
}
