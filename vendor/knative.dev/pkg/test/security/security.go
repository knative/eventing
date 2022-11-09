/*
Copyright 2022 The Knative Authors

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

package security

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/ptr"
)

var DefaultPodSecurityContext = corev1.PodSecurityContext{
	RunAsNonRoot: ptr.Bool(true),
	SeccompProfile: &corev1.SeccompProfile{
		Type: corev1.SeccompProfileTypeRuntimeDefault,
	},
}

var DefaultContainerSecurityContext = corev1.SecurityContext{
	AllowPrivilegeEscalation: ptr.Bool(false),
	Capabilities: &corev1.Capabilities{
		Drop: []corev1.Capability{"ALL"},
	},
}

// AllowRestrictedPodSecurityStandard adds SecurityContext to Pod and its containers so that it can run
// in a namespace with enforced "restricted" security standard.
func AllowRestrictedPodSecurityStandard(ctx context.Context, kubeClient kubernetes.Interface, pod *corev1.Pod) error {
	enforced, err := IsRestrictedPodSecurityEnforced(ctx, kubeClient, pod.Namespace)
	if err != nil {
		return err
	}
	if enforced {
		pod.Spec.SecurityContext = &DefaultPodSecurityContext
		for _, c := range pod.Spec.Containers {
			c.SecurityContext = &DefaultContainerSecurityContext
		}
	}
	return nil
}

// IsRestrictedPodSecurityEnforced checks if the given namespace has enforced restricted security standard.
func IsRestrictedPodSecurityEnforced(ctx context.Context, kubeClient kubernetes.Interface, namespace string) (bool, error) {
	ns, err := kubeClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	for k, v := range ns.Labels {
		if k == "pod-security.kubernetes.io/enforce" && v == "restricted" {
			return true, nil
		}
	}
	return false, nil
}
