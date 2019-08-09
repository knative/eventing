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

// This file contains higher-level encapsulated functions for writing cleaner eventing tests.

package common

import (
	"fmt"

	"knative.dev/eventing/test/base/resources"
	"knative.dev/pkg/test/helpers"
)

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
func (client *Client) CreateRBACResourcesForBrokers() {
	client.CreateServiceAccountOrFail(saIngressName)
	client.CreateServiceAccountOrFail(saFilterName)
	// The two RoleBindings are required for running Brokers correctly.
	client.CreateRoleBindingOrFail(
		saIngressName,
		crIngressName,
		fmt.Sprintf("%s-%s", saIngressName, crIngressName),
		client.Namespace,
	)
	client.CreateRoleBindingOrFail(
		saFilterName,
		crFilterName,
		fmt.Sprintf("%s-%s", saFilterName, crFilterName),
		client.Namespace,
	)
	// The two RoleBindings are required for access to shared configmaps for logging,
	// tracing, and metrics configuration.
	client.CreateRoleBindingOrFail(
		saIngressName,
		crConfigReaderName,
		fmt.Sprintf("%s-%s-%s", saIngressName, helpers.MakeK8sNamePrefix(client.Namespace), crConfigReaderName),
		resources.SystemNamespace,
	)
	client.CreateRoleBindingOrFail(
		saFilterName,
		crConfigReaderName,
		fmt.Sprintf("%s-%s-%s", saFilterName, helpers.MakeK8sNamePrefix(client.Namespace), crConfigReaderName),
		resources.SystemNamespace,
	)
}
