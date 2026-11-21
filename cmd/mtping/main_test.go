/*
Copyright 2026 The Knative Authors

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

package main

import (
	"os"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/yaml"

	"knative.dev/eventing/pkg/observability"
)

func TestDeploymentMetricsPortMatchesServedPort(t *testing.T) {
	b, err := os.ReadFile("../../config/core/deployments/pingsource-mt-adapter.yaml")
	if err != nil {
		t.Fatal("Failed to read the Deployment:", err)
	}
	var d appsv1.Deployment
	if err := yaml.Unmarshal(b, &d); err != nil {
		t.Fatal("Failed to decode the Deployment:", err)
	}

	for _, c := range d.Spec.Template.Spec.Containers {
		for _, p := range c.Ports {
			if p.Name == "metrics" {
				if p.ContainerPort != observability.DefaultMetricsPort {
					t.Errorf("Container %q declares metrics port %d, want %d", c.Name, p.ContainerPort, observability.DefaultMetricsPort)
				}
				return
			}
		}
	}
	t.Error("The Deployment declares no metrics port")
}
