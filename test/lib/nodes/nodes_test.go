/*
 * Copyright 2020 The Knative Authors
 *
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

// TODO(ksuszyns): remove the whole package after knative/pkg#1001 is merged

package nodes

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
)

func TestNodesClientRandomWorkerNode(t *testing.T) {
	masterNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "master-1",
			Labels: map[string]string{
				roleLabelFormat + "master": "",
			},
		},
	}
	workerNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-1",
		},
	}
	clientset := kubernetesfake.NewSimpleClientset(masterNode, workerNode)
	c := Client(clientset, newLogger())

	node, err := c.RandomWorkerNode()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	nodeName := workerNode.Name
	assertProperNodeIsReturned(t, node, nodeName)
}

func TestNodesClientRandomWorkerNode_OneNode(t *testing.T) {
	nodeName := "minikube"
	minikube := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
			Labels: map[string]string{
				roleLabelFormat + "master": "",
			},
		},
	}
	clientset := kubernetesfake.NewSimpleClientset(minikube)
	c := Client(clientset, newLogger())

	node, err := c.RandomWorkerNode()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	assertProperNodeIsReturned(t, node, nodeName)
}

func TestNodesClientRandomWorkerNode_NoNode(t *testing.T) {
	clientset := kubernetesfake.NewSimpleClientset()
	c := Client(clientset, newLogger())

	_, err := c.RandomWorkerNode()

	if err == nil {
		t.Fatal("Expected error, but not received")
	}
	actual := err.Error()
	if actual != "fetched 0 nodes, so can't chose a worker" {
		t.Errorf("Unexpected error: %s", actual)
	}
}

func assertProperNodeIsReturned(t *testing.T, node *corev1.Node, nodeName string) {
	t.Helper()
	if node == nil {
		t.Fatal("Expect not to be nil")
	}
	if node.Name != nodeName {
		t.Errorf("Got = %v, want: %s", node, nodeName)
	}
}
