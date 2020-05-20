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
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
)

func TestNodesClientGuessNodeExternalAddress(t *testing.T) {
	clientset := kubernetesfake.NewSimpleClientset()
	c := Client(clientset, newLogger())
	node := &corev1.Node{
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{{
				Type:    corev1.NodeInternalIP,
				Address: "10.0.2.17",
			}, {
				Type:    corev1.NodeHostName,
				Address: "node-17",
			}, {
				Type:    corev1.NodeInternalDNS,
				Address: "node-17.europe3.internal",
			}, {
				Type:    corev1.NodeExternalIP,
				Address: "35.123.11.234",
			}},
		},
	}

	address := c.GuessNodeExternalAddress(node)

	if address.Address != "35.123.11.234" {
		t.Errorf("Address: %s want: 35.123.11.234", address.Address)
	}
}

func TestNodesClientGuessNodeExternalAddress_PrivateOnly(t *testing.T) {
	clientset := kubernetesfake.NewSimpleClientset()
	c := Client(clientset, newLogger())
	node := &corev1.Node{
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{{
				Type:    corev1.NodeInternalIP,
				Address: "10.0.2.17",
			}, {
				Type:    corev1.NodeHostName,
				Address: "node-17",
			}, {
				Type:    corev1.NodeInternalDNS,
				Address: "node-17.europe3.internal",
			}},
		},
	}

	address := c.GuessNodeExternalAddress(node)

	if address.Address != "10.0.2.17" {
		t.Errorf("Address: %s want: 10.0.2.17", address.Address)
	}
}
