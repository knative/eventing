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
	corev1 "k8s.io/api/core/v1"
)

const (
	roleLabelFormat = "node-role.kubernetes.io/"
)

// FilterOutByRole returns a new slice without nodes that have given role
func FilterOutByRole(nodes []corev1.Node, role string) []corev1.Node {
	result := make([]corev1.Node, 0, len(nodes))
	for _, node := range nodes {
		if !hasRole(role, node) {
			result = append(result, node)
		}
	}
	return result
}

func hasRole(role string, node corev1.Node) bool {
	if node.Labels == nil {
		return false
	}
	label := roleLabelFormat + role
	_, has := node.Labels[label]
	return has
}
