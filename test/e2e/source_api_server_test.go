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

import (
	"fmt"
	"testing"

	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/test/base/resources"
	"knative.dev/eventing/test/common"

	eventingtesting "knative.dev/eventing/pkg/reconciler/testing"
)

func TestApiServerSource(t *testing.T) {
	const (
		apiServerSourceName = "e2e-api-server-source"

		clusterRoleName    = "event-watcher-cr"
		serviceAccountName = "event-watcher-sa"
		helloworldPodName  = "e2e-api-server-source-helloworld-pod"
		loggerPodName      = "e2e-api-server-source-logger-pod"
	)

	client := setup(t, true)
	defer tearDown(client)

	// creates ServiceAccount and ClusterRoleBinding with default cluster-admin role
	cr := resources.EventWatcherClusterRole(clusterRoleName)
	client.CreateServiceAccountOrFail(serviceAccountName)
	client.CreateClusterRoleOrFail(cr)
	client.CreateClusterRoleBindingOrFail(
		serviceAccountName,
		clusterRoleName,
		fmt.Sprintf("%s-%s", serviceAccountName, clusterRoleName),
	)

	// create event logger pod and service
	loggerPod := resources.EventLoggerPod(loggerPodName)
	client.CreatePodOrFail(loggerPod, common.WithService(loggerPodName))

	// create the ApiServerSource
	// apiServerSourceResources is the list of resources to watch for this ApiServerSource
	apiServerSourceResources := []sourcesv1alpha1.ApiServerResource{
		{
			APIVersion: "v1",
			Kind:       "Event",
		},
	}
	// mode is the watch mode: `Ref` sends only the reference to the resource, `Resource` sends the full resource.
	mode := "Ref"
	apiServerSource := eventingtesting.NewApiServerSource(
		apiServerSourceName,
		client.Namespace,
		eventingtesting.WithApiServerSourceSpec(sourcesv1alpha1.ApiServerSourceSpec{
			Resources:          apiServerSourceResources,
			ServiceAccountName: serviceAccountName,
			Mode:               mode,
			Sink:               resources.ServiceRef(loggerPodName),
		}),
	)

	client.CreateApiServerSourceOrFail(apiServerSource)

	// wait for all test resources to be ready
	if err := client.WaitForAllTestResourcesReady(); err != nil {
		t.Fatalf("Failed to get all test resources ready: %v", err)
	}

	helloworldPod := resources.HelloWorldPod(helloworldPodName)
	client.CreatePodOrFail(helloworldPod)

	// verify the logger service receives the event(s)
	// TODO(Fredy-Z): right now it's only doing a very basic check by looking for the "Event" word,
	//                we can add a json matcher to improve it in the future.
	data := "Event"
	if err := client.CheckLog(loggerPodName, common.CheckerContains(data)); err != nil {
		t.Fatalf("String %q does not appear in logs of logger pod %q: %v", data, loggerPodName, err)
	}
}
