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

package resources

import "fmt"

const (
	DefaultBrokerName = "default"

	FilterServiceAccountName = "eventing-broker-filter"
	FilterRoleBindingName    = "eventing-broker-filter"
	FilterClusterRoleName    = "eventing-broker-filter"

	IngressServiceAccountName = "eventing-broker-ingress"
	IngressRoleBindingName    = "eventing-broker-ingress"
	IngressClusterRoleName    = "eventing-broker-ingress"

	ConfigClusterRoleName = "eventing-config-reader"
)

// ConfigRoleBindingName returns a name for a RoleBinding allowing access to the
// shared ConfigMaps from a service account in another namespace. Because these
// are all created in the system namespace, they must be named for their
// subject namespace.
func ConfigRoleBindingName(saName, ns string) string {
	return fmt.Sprintf("eventing-config-reader-%s-%s", ns, saName)
}
