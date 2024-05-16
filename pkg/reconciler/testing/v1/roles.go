/*
Copyright 2024 The Knative Authors

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

package testing

import (
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RoleOption enables further configuration of a Role.
type RoleOption func(*v1.Role)
type PolicyRuleOption func(*v1.PolicyRule)

// createRole creates a Role in the Kubernetes cluster.
func CreateRole(name, namespace string, o ...RoleOption) *v1.Role {
	r := &v1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Rules: []v1.PolicyRule{},
	}
	for _, opt := range o {
		opt(r)
	}
	return r
}

func WithRoleRules(rules ...*v1.PolicyRule) RoleOption {
	roleRules := make([]v1.PolicyRule, 0, len(rules))
	for _, rule := range rules {
		roleRules = append(roleRules, *rule)
	}
	return func(r *v1.Role) {
		r.Rules = roleRules
	}
}

func WithPolicyRule(o ...PolicyRuleOption) *v1.PolicyRule {
	pr := &v1.PolicyRule{}
	for _, opt := range o {
		opt(pr)
	}
	return pr
}

func WithAPIGroups(apiGroups []string) PolicyRuleOption {
	return func(r *v1.PolicyRule) {
		r.APIGroups = apiGroups
	}
}

func WithResources(resources ...string) PolicyRuleOption {
	return func(r *v1.PolicyRule) {
		r.Resources = resources
	}
}

func WithVerbs(verbs ...string) PolicyRuleOption {
	return func(r *v1.PolicyRule) {
		r.Verbs = verbs
	}
}
