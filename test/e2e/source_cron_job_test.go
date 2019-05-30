// +build e2e

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

package e2e

import "testing"

func TestCronJobSource(t *testing.T) {
	const (
		cronJobSourceName = "e2e-cron-job-source"
		schedule          = "0/30 * * * * ? *"

		loggerPodName = "e2e-cron-job-source-logger-pod"
		saIngressName = "eventing-broker-ingress"
		crIngressName = "eventing-broker-ingress"
	)

	client := Setup(t, "", true)
	defer TearDown(client)

	// creates ServiceAccount and ClusterRoleBinding with default cluster-admin role
	if err := client.CreateServiceAccountAndBinding(saIngressName, crIngressName); err != nil {
		t.Fatalf("Failed to create the Ingress ServiceAccount and ServiceAccountRoleBinding: %v", err)
	}
	if err := client.CreateServiceAccountAndBinding(saFilterName, crFilterName); err != nil {
		t.Fatalf("Failed to create the Filter ServiceAccount and ServiceAccountRoleBinding: %v", err)
	}
}
