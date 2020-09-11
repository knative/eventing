/*
 * Copyright 2020 The Knative Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package duck

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/pkg/test"
)

// WaitForKServiceReady will wait until ksvc reports that's ready
func WaitForKServiceReady(client resources.ServingClient, name, namespace string) error {
	meta := resources.NewMetaResource(name, namespace, &resources.KServiceType)
	return WaitForResourceReady(client.Dynamic, meta)
}

// WaitForKServiceScales will wait until ksvc scale is satisfied
func WaitForKServiceScales(ctx context.Context, client resources.ServingClient, name, namespace string, satisfyScale func(int) bool) error {
	err := WaitForKServiceReady(client, name, namespace)
	if err != nil {
		return err
	}
	deploymentName, err := waitForKServiceDeploymentName(client, name, namespace)
	if err != nil {
		return err
	}
	inState := func(dep *appsv1.Deployment) (bool, error) {
		return satisfyScale(int(dep.Status.ReadyReplicas)), nil
	}
	return test.WaitForDeploymentState(
		ctx, client.Kube, deploymentName, inState, "scales", namespace, timeout,
	)
}

func waitForKServiceDeploymentName(client resources.ServingClient, name, namespace string) (string, error) {
	var deploymentName string
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		dn, found, err := resources.KServiceDeploymentName(client, name, namespace)
		if found {
			deploymentName = dn
		}
		return found, err
	})

	return deploymentName, err
}
