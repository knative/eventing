/*
Copyright 2021 The Knative Authors

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
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing"
)

func TestMakePingAdapter(t *testing.T) {
	args := Args{
		ConfigEnvVars:   (&reconcilersource.EmptyVarsGenerator{}).ToEnvVars(),
		NoShutdownAfter: 40,
		SinkTimeout:     48,
	}

	want := []corev1.EnvVar{{
		Name: system.NamespaceEnvKey,
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.namespace",
			},
		},
	}, {
		Name:  "K_LEADER_ELECTION_CONFIG",
		Value: "",
	}, {
		Name:  "K_NO_SHUTDOWN_AFTER",
		Value: "40",
	}, {
		Name:  "K_SINK_TIMEOUT",
		Value: "48",
	}, {
		Name:  "K_LOGGING_CONFIG",
		Value: "",
	}, {
		Name:  "K_METRICS_CONFIG",
		Value: "",
	}, {
		Name:  "K_TRACING_CONFIG",
		Value: "",
	}}

	got := MakeReceiveAdapterEnvVar(args)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Error("unexpected condition (-want, +got) =", diff)
	}
}
