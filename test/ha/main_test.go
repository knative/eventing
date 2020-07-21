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

package ha

import (
	"os"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/eventing/test"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/pkg/system"
)

var setup = testlib.Setup
var tearDown = testlib.TearDown

func TestMain(m *testing.M) {
	test.InitializeEventingFlags()
	os.Exit(m.Run())
}

func setDeploymentReplicas(client *testlib.Client, name string, replicas int32) error {
	scale, err := client.Kube.Kube.AppsV1().Deployments(system.Namespace()).GetScale(name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	scale.Spec.Replicas = replicas
	_, err = client.Kube.Kube.AppsV1().Deployments(system.Namespace()).UpdateScale(name, scale)

	if err != nil {
		return err
	}

	// Wait for replicas to be ready
	err = wait.PollImmediate(2*time.Second, 10*time.Second, func() (done bool, err error) {
		d, err := client.Kube.Kube.AppsV1().Deployments(system.Namespace()).Get(name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return d.Status.ReadyReplicas == replicas, nil
	})

	return nil
}
