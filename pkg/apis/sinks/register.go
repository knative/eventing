/*
Copyright 2024 The Knative Authors

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

package sinks

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

const (
	GroupName = "sinks.knative.dev"
)

var (
	// JobSinkResource respresents a Knative Eventing sink JobSink
	JobSinkResource = schema.GroupResource{
		Group:    GroupName,
		Resource: "jobsinks",
	}
)

type Config struct {
	KubeClient kubernetes.Interface
}

type configKey struct{}

func WithConfig(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, configKey{}, cfg)
}

func GetConfig(ctx context.Context) *Config {
	v := ctx.Value(configKey{})
	if v == nil {
		panic("Missing value for config")
	}
	return v.(*Config)
}
