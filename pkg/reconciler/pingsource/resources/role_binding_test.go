/*
Copyright 2020 The Knative Authors

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

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"

	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
)

func TestNewRoleBinding(t *testing.T) {
	const (
		rbName = "my-test-role-binding"
		crName = "my-test-cluster-role"
		testNS = "test-ns"
	)
	src := &v1alpha2.PingSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-name",
			Namespace: testNS,
			UID:       "source-uid",
		},
		Spec: v1alpha2.PingSourceSpec{
			Schedule: "*/2 * * * *",
			JsonData: "data",
		},
	}

	want := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbName,
			Namespace: testNS,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(src),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     crName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: testNS,
				Name:      rbName,
			},
		},
	}

	got := MakeRoleBinding(src, rbName, crName)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected condition (-want, +got) = %v", diff)
	}
}
