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
	"encoding/json"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
	pkgTest "knative.dev/pkg/test"
)

// PodOption enables further configuration of a Pod.
type PodOption func(*corev1.Pod)

// Option enables further configuration of a ClusterRole.
type ClusterRoleOption func(*rbacv1.ClusterRole)

// EventSenderPod creates a Pod that sends a single event to the given address.
func EventSenderPod(name string, sink string, event *CloudEvent) (*corev1.Pod, error) {
	const imageName = "sendevents"
	if event.Encoding == "" {
		event.Encoding = CloudEventEncodingBinary
	}
	eventExtensionsBytes, error := json.Marshal(event.Extensions)
	eventExtensions := string(eventExtensionsBytes)
	if error != nil {
		return nil, fmt.Errorf("encountered error when we marshall cloud event extensions %v", error)
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            imageName,
				Image:           pkgTest.ImagePath(imageName),
				ImagePullPolicy: corev1.PullAlways,
				Args: []string{
					"-event-id",
					event.ID,
					"-event-type",
					event.Type,
					"-event-source",
					event.Source,
					"-event-extensions",
					eventExtensions,
					"-event-data",
					event.Data,
					"-event-encoding",
					event.Encoding,
					"-sink",
					sink,
				},
			}},
			//TODO restart on failure?
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}, nil
}

// EventLoggerPod creates a Pod that logs events received.
func EventLoggerPod(name string) *corev1.Pod {
	const imageName = "logevents"
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"e2etest": string(uuid.NewUUID())},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            imageName,
				Image:           pkgTest.ImagePath(imageName),
				ImagePullPolicy: corev1.PullAlways,
			}},
			RestartPolicy: corev1.RestartPolicyAlways,
		},
	}
}

// EventTransformationPod creates a Pod that transforms events received.
func EventTransformationPod(name string, event *CloudEvent) *corev1.Pod {
	const imageName = "transformevents"
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"e2etest": string(uuid.NewUUID())},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            imageName,
				Image:           pkgTest.ImagePath(imageName),
				ImagePullPolicy: corev1.PullAlways,
				Args: []string{
					"-event-type",
					event.Type,
					"-event-source",
					event.Source,
					"-event-data",
					event.Data,
				},
			}},
			RestartPolicy: corev1.RestartPolicyAlways,
		},
	}
}

// HelloWorldPod creates a Pod that logs "Hello, World!".
func HelloWorldPod(name string, options ...PodOption) *corev1.Pod {
	const imageName = "helloworld"
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            imageName,
				Image:           pkgTest.ImagePath(imageName),
				ImagePullPolicy: corev1.PullAlways,
			}},
			RestartPolicy: corev1.RestartPolicyNever,
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

// SequenceStepperPod creates a Pod that can be used as a step in testing Sequence.
// Note event data used in the test must be CloudEventBaseData, and this Pod as a Subscriber will receive the event,
// and return a new event with eventMsgAppender added to data.Message.
func SequenceStepperPod(name, eventMsgAppender string) *corev1.Pod {
	const imageName = "sequencestepper"
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"e2etest": string(uuid.NewUUID())},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            imageName,
				Image:           pkgTest.ImagePath(imageName),
				ImagePullPolicy: corev1.PullAlways,
				Args: []string{
					"-msg-appender",
					eventMsgAppender,
				},
			}},
			RestartPolicy: corev1.RestartPolicyAlways,
		},
	}
}

// EventFilteringPod creates a Pod that either filter or send the received CloudEvent
func EventFilteringPod(name string, filter bool) *corev1.Pod {
	const imageName = "filterevents"
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"e2etest": string(uuid.NewUUID())},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            imageName,
				Image:           pkgTest.ImagePath(imageName),
				ImagePullPolicy: corev1.PullAlways,
			}},
			RestartPolicy: corev1.RestartPolicyAlways,
		},
	}
	if filter {
		pod.Spec.Containers[0].Args = []string{"-filter"}
	}
	return pod
}

// EventLatencyPod creates a Pod that measures events transfer latency.
func EventLatencyPod(name, sink string, eventCount int) *corev1.Pod {
	const imageName = "latency"
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"perftest": string(uuid.NewUUID())},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            imageName,
				Image:           pkgTest.ImagePath(imageName),
				ImagePullPolicy: corev1.PullAlways,
				Args: []string{
					"-sink",
					sink,
					"-event-count",
					strconv.Itoa(eventCount),
				},
			}},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
}

// Service creates a Kubernetes Service with the given name, namespace, and
// selector. Port 8080 is set as the target port.
func Service(name string, selector map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.ServiceSpec{
			Selector: selector,
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       80,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt(8080),
			}},
		},
	}
}

// ServiceRef returns a Service ObjectReference for a given Service name.
func ServiceRef(name string) *corev1.ObjectReference {
	return pkgTest.CoreV1ObjectReference(ServiceKind, CoreAPIVersion, name)
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
// namespace, ClusterRole name, RoleBinding name and namespace.
func RoleBinding(saName, saNamespace, crName, rbName, rbNamespace string) *rbacv1.RoleBinding {
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
			Kind:     "ClusterRole",
			Name:     crName,
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

// EventWatcherClusterRole creates a Kubernetes ClusterRole
func ClusterRole(crName string, options ...ClusterRoleOption) *rbacv1.ClusterRole {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: crName,
		},
		Rules: []rbacv1.PolicyRule{},
	}
	for _, option := range options {
		option(clusterRole)
	}
	return clusterRole
}

// WithRuleForClusterRole is a ClusterRole Option for adding a rule
func WithRuleForClusterRole(rule *rbacv1.PolicyRule) ClusterRoleOption {
	return func(cr *rbacv1.ClusterRole) {
		cr.Rules = append(cr.Rules, *rule)
	}
}

// EventWatcherClusterRole creates a Kubernetes ClusterRole that can be used to watch Events.
func EventWatcherClusterRole(crName string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: crName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{rbacv1.APIGroupAll},
				Resources: []string{"events"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
}
