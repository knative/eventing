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

package common

// This file contains higher-level encapsulated functions for writing cleaner eventing tests.

import "github.com/knative/eventing/test/base/resources"

const (
	// the two ServiceAccounts are required for creating new Brokers in the current namespace
	saIngressName = "eventing-broker-ingress"
	saFilterName  = "eventing-broker-filter"

	// the three ClusterRoles are preinstalled in Knative Eventing setup
	crIngressName      = "eventing-broker-ingress"
	crFilterName       = "eventing-broker-filter"
	crConfigReaderName = "eventing-config-reader"
)

// CreateRBACResourcesForBrokers creates required RBAC resources for creating Brokers,
// see https://github.com/knative/docs/blob/master/docs/eventing/broker-trigger.md - Manual Setup.
func CreateRBACResourcesForBrokers(client *Client) {
	client.CreateServiceAccountOrFail(saIngressName)
	client.CreateServiceAccountOrFail(saFilterName)
	client.CreateClusterRoleBindingOrFail(saIngressName, crIngressName, client.Namespace)
	client.CreateClusterRoleBindingOrFail(saIngressName, crConfigReaderName, resources.SystemNamespace)
	client.CreateClusterRoleBindingOrFail(saFilterName, crFilterName, client.Namespace)
	client.CreateClusterRoleBindingOrFail(saFilterName, crConfigReaderName, resources.SystemNamespace)
}
