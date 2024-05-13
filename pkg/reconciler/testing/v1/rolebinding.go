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

// RoleBindingOption enables further configuration of a RoleBinding.
type RoleBindingOption func(*v1.RoleBinding)
type SubjectOption func(*v1.Subject)
type RoleRefOption func(*v1.RoleRef)

func CreateRoleBinding(roleName, namespace string, o ...RoleBindingOption) *v1.RoleBinding {
	rb := &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName + "-binding",
			Namespace: namespace,
		},
		Subjects: []v1.Subject{},
		RoleRef:  v1.RoleRef{},
	}
	for _, opt := range o {
		opt(rb)
	}
	return rb
}

func WithRoleBindingSubjects(subjects ...*v1.Subject) RoleBindingOption {
	s := make([]v1.Subject, 0, len(subjects))
	for _, subject := range subjects {
		s = append(s, *subject)
	}
	return func(rb *v1.RoleBinding) {
		rb.Subjects = s
	}
}

func WithSubjects(o ...SubjectOption) *v1.Subject {
	s := &v1.Subject{}
	for _, opt := range o {
		opt(s)
	}
	return s
}

func WithSubjectKind(kind string) SubjectOption {
	return func(s *v1.Subject) {
		s.Kind = kind
	}
}

func WithSubjectName(name string) SubjectOption {
	return func(s *v1.Subject) {
		s.Name = name
	}
}

func WithRoleBindingRoleRef(roleRef *v1.RoleRef) RoleBindingOption {
	return func(rb *v1.RoleBinding) {
		rb.RoleRef = *roleRef
	}
}

func WithRoleRef(o ...RoleRefOption) *v1.RoleRef {
	rr := &v1.RoleRef{}
	for _, opt := range o {
		opt(rr)
	}
	return rr
}

func WithRoleRefAPIGroup(apiGroup string) RoleRefOption {
	return func(rr *v1.RoleRef) {
		rr.APIGroup = apiGroup
	}
}

func WithRoleRefKind(kind string) RoleRefOption {
	return func(rr *v1.RoleRef) {
		rr.Kind = kind
	}
}

func WithRoleRefName(name string) RoleRefOption {
	return func(rr *v1.RoleRef) {
		rr.Name = name
	}
}
