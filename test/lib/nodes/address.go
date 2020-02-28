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
	"sort"

	corev1 "k8s.io/api/core/v1"
)

// GuessNodeExternalAddress tries to guess external address of a node
func (n *NodesClient) GuessNodeExternalAddress(node *corev1.Node) *corev1.NodeAddress {
	sorted := make([]*corev1.NodeAddress, len(node.Status.Addresses))
	for i := 0; i < len(node.Status.Addresses); i++ {
		sorted[i] = &node.Status.Addresses[i]
	}
	sort.Sort(byAddressType{sorted})
	first := sorted[0]
	priority := addressTypePriority[first.Type]
	if priority >= 2 {
		n.logger.Warnf("Chosen address is probably an internal type: %s, "+
			"and might be unaccessible", first.Type)
	}
	return first
}

type nodeAddresses []*corev1.NodeAddress

type byAddressType struct {
	nodeAddresses
}

func (s byAddressType) Len() int {
	return len(s.nodeAddresses)
}

func (s byAddressType) Swap(i, j int) {
	s.nodeAddresses[i], s.nodeAddresses[j] = s.nodeAddresses[j], s.nodeAddresses[i]
}

func (s byAddressType) Less(i, j int) bool {
	return addressTypePriority[s.nodeAddresses[i].Type] <
		addressTypePriority[s.nodeAddresses[j].Type]
}

var addressTypePriority = map[corev1.NodeAddressType]int{
	corev1.NodeExternalDNS: 0,
	corev1.NodeExternalIP:  1,
	corev1.NodeInternalIP:  2,
	corev1.NodeInternalDNS: 3,
	corev1.NodeHostName:    4,
}
