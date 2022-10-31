package security

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/ptr"
)

var DefaultPodSecurityContext = corev1.PodSecurityContext{
	RunAsNonRoot: ptr.Bool(true),
}

var DefaultContainerSecurityContext = corev1.SecurityContext{
	AllowPrivilegeEscalation: ptr.Bool(false),
	Capabilities: &corev1.Capabilities{
		Drop: []corev1.Capability{"ALL"},
	},
}

func AllowRestrictedPodSecurityStandard(ctx context.Context, kubeClient kubernetes.Interface, pod *corev1.Pod) error {
	ns, err := kubeClient.CoreV1().Namespaces().Get(ctx, pod.Namespace, metav1.GetOptions{})
	if err != nil {
		return err
	}
	for k, v := range ns.Labels {
		if k == "pod-security.kubernetes.io/enforce" && v == "restricted" {
			pod.Spec.SecurityContext = &DefaultPodSecurityContext
			for _, c := range pod.Spec.Containers {
				c.SecurityContext = &DefaultContainerSecurityContext
			}
		}
	}
	return nil
}
