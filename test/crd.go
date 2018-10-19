/*
Copyright 2018 The Knative Authors
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

package test

// crd contains functions that construct boilerplate CRD definitions.

import (
	"github.com/knative/serving/pkg/apis/serving/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	rbacV1beta1 "k8s.io/api/rbac/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Route returns a Route object in namespace
func Route(name string, namespace string, configName string) *v1alpha1.Route {
	return &v1alpha1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: v1alpha1.RouteSpec{
			Traffic: []v1alpha1.TrafficTarget{
				v1alpha1.TrafficTarget{
					ConfigurationName: configName,
					Percent:           100,
				},
			},
		},
	}
}

// Configuration returns a Configuration object in namespace with the name names.Config
// that uses the image specifed by imagePath.
func Configuration(name string, namespace string, imagePath string) *v1alpha1.Configuration {
	return &v1alpha1.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ConfigurationSpec{
			RevisionTemplate: v1alpha1.RevisionTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"knative.dev/type": "container"},
				},
				Spec: v1alpha1.RevisionSpec{
					Container: corev1.Container{
						Image: imagePath,
					},
				},
			},
		},
	}
}

// ServiceAccount returns ServiceAccount object in given namespace
func ServiceAccount(name string, namespace string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// ClusterRoleBinding create ClusterRoleBinding for given subject and role
func ClusterRoleBinding(name string, namespace string, serviceAccount string, role string) *rbacV1beta1.ClusterRoleBinding {
	return &rbacV1beta1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Subjects: []rbacV1beta1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount,
				Namespace: namespace,
			},
		},
		RoleRef: rbacV1beta1.RoleRef{
			Kind:     "ClusterRole",
			Name:     role,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
}

// NGinxPod returns nginx pod defined in given namespace
func NGinxPod(namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx",
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:1.7.9",
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 80,
						},
					},
				},
			},
		},
	}
}
