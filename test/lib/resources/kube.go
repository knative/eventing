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

// This file contains functions that construct common Kubernetes resources.

import (
	"fmt"

	v1 "knative.dev/pkg/apis/duck/v1"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	pkgTest "knative.dev/pkg/test"
)

// PodOption enables further configuration of a Pod.
type PodOption func(*corev1.Pod)

// Option enables further configuration of a Role.
type RoleOption func(*rbacv1.Role)

// HelloWorldPod creates a Pod that logs "Hello, World!".
func HelloWorldPod(name string, options ...PodOption) *corev1.Pod {
	const imageName = "print"
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            imageName,
				Image:           pkgTest.ImagePath(imageName),
				ImagePullPolicy: corev1.PullIfNotPresent,
			}},
			RestartPolicy: corev1.RestartPolicyAlways,
		},
	}
	for _, option := range options {
		option(pod)
	}
	return pod
}

// WithLabelsForPod returns an option setting the pod labels
func WithLabelsForPod(labels map[string]string) PodOption {
	return func(p *corev1.Pod) {
		p.Labels = labels
	}
}

const (
	PerfConsumerService   = "perf-consumer"
	PerfAggregatorService = "perf-aggregator"
	PerfServiceAccount    = "perf-eventing"
)

func PerformanceConsumerService() *corev1.Service {
	return Service(
		PerfConsumerService,
		map[string]string{"role": "perf-consumer"},
		[]corev1.ServicePort{{
			Protocol:   corev1.ProtocolTCP,
			Port:       80,
			TargetPort: intstr.FromString("cloudevents"),
			Name:       "http",
		}},
	)
}

func PerformanceAggregatorService() *corev1.Service {
	return Service(
		PerfAggregatorService,
		map[string]string{"role": "perf-aggregator"},
		[]corev1.ServicePort{{
			Protocol:   corev1.ProtocolTCP,
			Port:       10000,
			TargetPort: intstr.FromString("grpc"),
			Name:       "grpc",
		}},
	)
}

func PerformanceImageReceiverPod(imageName string, pace string, warmup string, aggregatorHostname string, additionalArgs ...string) *corev1.Pod {
	const podName = "perf-receiver"

	args := append([]string{
		"--roles=receiver",
		fmt.Sprintf("--pace=%s", pace),
		fmt.Sprintf("--warmup=%s", warmup),
		fmt.Sprintf("--aggregator=%s:10000", aggregatorHostname),
	}, additionalArgs...)

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"role": "perf-consumer",
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: PerfServiceAccount,
			RestartPolicy:      corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:  "receiver",
				Image: pkgTest.ImagePath(imageName),
				Args:  args,
				Env: []corev1.EnvVar{{
					Name: "POD_NAME",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				}, {
					Name: "POD_NAMESPACE",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.namespace",
						},
					},
				}},
				Ports: []corev1.ContainerPort{{
					Name:          "cloudevents",
					ContainerPort: 8080,
				}},
			}},
		},
	}
}

func PerformanceImageAggregatorPod(expectedRecords int, publish bool, additionalArgs ...string) *corev1.Pod {
	const podName = "perf-aggregator"
	const imageName = "performance"

	args := append([]string{
		"--roles=aggregator",
		fmt.Sprintf("--publish=%v", publish),
		fmt.Sprintf("--expect-records=%d", expectedRecords),
	}, additionalArgs...)

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"role": "perf-aggregator",
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: PerfServiceAccount,
			RestartPolicy:      corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:  "aggregator",
				Image: pkgTest.ImagePath(imageName),
				Args:  args,
				Env: []corev1.EnvVar{{
					Name: "POD_NAMESPACE",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.namespace",
						},
					},
				}},
				TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
				Ports: []corev1.ContainerPort{{
					Name:          "grpc",
					ContainerPort: 10000,
				}},
			}},
		},
	}
}

// Service creates a Kubernetes Service with the given name, selector and ports
func Service(name string, selector map[string]string, ports []corev1.ServicePort) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.ServiceSpec{
			Selector: selector,
			Ports:    ports,
		},
	}
}

// Service creates a Kubernetes Service with the given name, namespace, and
// selector. Port 8080 is set as the target port.
func ServiceDefaultHTTP(name string, selector map[string]string) *corev1.Service {
	return Service(name, selector, []corev1.ServicePort{{
		Name:       "http",
		Port:       80,
		Protocol:   corev1.ProtocolTCP,
		TargetPort: intstr.FromInt(8080),
	}})
}

// ServiceRef returns a Service ObjectReference for a given Service name.
func ServiceRef(name string) *corev1.ObjectReference {
	return pkgTest.CoreV1ObjectReference(ServiceKind, CoreAPIVersion, name)
}

// ServiceKRef returns a Service ObjectReference for a given Service name.
func ServiceKRef(name string) *v1.KReference {
	ref := pkgTest.CoreV1ObjectReference(ServiceKind, CoreAPIVersion, name)
	return &v1.KReference{
		Kind:       ref.Kind,
		Namespace:  ref.Namespace,
		Name:       ref.Name,
		APIVersion: ref.APIVersion,
	}
}

// ServiceAccount creates a Kubernetes ServiceAccount with the given name and namespace.
func ServiceAccount(name, namespace string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// RoleBinding creates a Kubernetes RoleBinding with the given ServiceAccount name and
// namespace, Role or ClusterRole Kind, name, RoleBinding name and namespace.
func RoleBinding(saName, saNamespace, rKind, rName, rbName, rbNamespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbName,
			Namespace: rbNamespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: saNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     rKind,
			Name:     rName,
			APIGroup: rbacv1.SchemeGroupVersion.Group,
		},
	}
}

// ClusterRoleBinding creates a Kubernetes ClusterRoleBinding with the given ServiceAccount name and
// namespace, ClusterRole name, ClusterRoleBinding name.
func ClusterRoleBinding(saName, saNamespace, crName, crbName string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: crbName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: saNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     crName,
			APIGroup: rbacv1.SchemeGroupVersion.Group,
		},
	}
}

// EventWatcherRole creates a Kubernetes Role
func Role(rName string, options ...RoleOption) *rbacv1.Role {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name: rName,
		},
		Rules: []rbacv1.PolicyRule{},
	}
	for _, option := range options {
		option(role)
	}
	return role
}

// WithRuleForRole is a Role Option for adding a rule
func WithRuleForRole(rule *rbacv1.PolicyRule) RoleOption {
	return func(r *rbacv1.Role) {
		r.Rules = append(r.Rules, *rule)
	}
}
