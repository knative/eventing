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
	"errors"
	"math/rand"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NodesClient contains interface for making requests to kubernetes client.
type NodesClient struct {
	kube   kubernetes.Interface
	logger *zap.SugaredLogger
}

// Client creates a new nodes client
func Client(kube kubernetes.Interface, logger *zap.SugaredLogger) *NodesClient {
	return &NodesClient{
		kube:   kube,
		logger: logger,
	}
}

// RandomWorkerNode gets a worker node randomly
func (n *NodesClient) RandomWorkerNode() (*corev1.Node, error) {
	nodes, err := n.kube.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	nodesCount := len(nodes.Items)
	if nodesCount == 0 {
		return nil, errors.New("fetched 0 nodes, so can't chose a worker")
	}
	if nodesCount == 1 {
		node := nodes.Items[0]
		n.logger.Infof("Only one node found (named: %s), returning it as"+
			" it must be a worker (Minikube, CRC)", node.Name)
		return &node, nil
	} else {
		role := "master"
		n.logger.Infof("Filtering %d nodes, to not contain role: %s", nodesCount, role)
		workers := FilterOutByRole(nodes.Items, role)
		n.logger.Infof("Found %d worker(s)", len(workers))
		worker := workers[rand.Intn(len(workers))]
		n.logger.Infof("Chosen node: %s", worker.Name)
		return &worker, nil
	}
}
