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

package resources

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/eventing/pkg/adapter/mtping"
	"knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/pkg/system"
)

type Args struct {
	MetricsConfig   string
	LoggingConfig   string
	LeConfig        string
	NoShutdownAfter int
	SinkTimeout     int
}

// MakeReceiveAdapterEnvVar generates the environment variables for the pingsources
func MakeReceiveAdapterEnvVar(args Args) []corev1.EnvVar {
	return []corev1.EnvVar{{
		Name: system.NamespaceEnvKey,
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.namespace",
			},
		},
	}, {
		Name:  adapter.EnvConfigMetricsConfig,
		Value: args.MetricsConfig,
	}, {
		Name:  adapter.EnvConfigLoggingConfig,
		Value: args.LoggingConfig,
	}, {
		Name:  adapter.EnvConfigLeaderElectionConfig,
		Value: args.LeConfig,
	}, {
		Name:  mtping.EnvNoShutdownAfter,
		Value: strconv.Itoa(args.NoShutdownAfter),
	}, {
		Name:  adapter.EnvSinkTimeout,
		Value: strconv.Itoa(args.SinkTimeout),
	}}

}
