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
	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
)

// PingSourceOption enables further configuration of a CronJob.
type PingSourceOption func(*v1alpha2.PingSource)

// NewPingSource creates a PingSource with PingSourceOption.
func NewPingSource(name, namespace string, o ...PingSourceOption) *v1alpha2.PingSource {
	c := &v1alpha2.PingSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range o {
		opt(c)
	}
	c.SetDefaults(context.Background()) // TODO: We should add defaults and validation.
	return c
}

func WithPingSourceSpec(spec v1alpha2.PingSourceSpec) PingSourceOption {
	return func(c *v1alpha2.PingSource) {
		c.Spec = spec
	}
}
