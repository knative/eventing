package resources

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/network"
)

const (
	PortName           = "http"
	PortNumber         = 80
	MessagingRoleLabel = "messaging.knative.dev/role"
	MessagingRole      = "in-memory-channel"
)

// ServiceOption can be used to optionally modify the K8s service in CreateK8sService
type K8sServiceOption func(*corev1.Service) error

func CreateChannelServiceName(name string) string {
	return fmt.Sprintf("%s-kn-channel", name)
}

// ExternalService is a functional option for CreateK8sService to create a K8s service of type ExternalName
// pointing to the specified service in a namespace.
func ExternalService(namespace, service string) K8sServiceOption {
	return func(svc *corev1.Service) error {
		svc.Spec = corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: network.GetServiceHostname(service, namespace),
		}
		return nil
	}
}

// NewK8sService creates a new Service for a Channel resource. It also sets the appropriate
// OwnerReferences on the resource so handleObject can discover the Channel resource that 'owns' it.
// As well as being garbage collected when the Channel is deleted.
func NewK8sService(imc *v1.InMemoryChannel, opts ...K8sServiceOption) (*corev1.Service, error) {
	// Add annotations
	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      CreateChannelServiceName(imc.ObjectMeta.Name),
			Namespace: imc.Namespace,
			Labels: map[string]string{
				MessagingRoleLabel: MessagingRole,
			},
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(imc),
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     PortName,
					Protocol: corev1.ProtocolTCP,
					Port:     PortNumber,
				},
			},
		},
	}
	for _, opt := range opts {
		if err := opt(svc); err != nil {
			return nil, err
		}
	}
	return svc, nil
}
