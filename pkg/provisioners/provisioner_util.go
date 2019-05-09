package provisioners

import (
	"context"

	"github.com/knative/pkg/kmeta"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	"fmt"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/pkg/logging"
	"github.com/knative/pkg/system"
)

// ServiceOption can be used to optionally modify the K8s default that gets created for the Dispatcher in CreateDispatcherService
type ServiceOption func(*v1.Service) error

func CreateDispatcherService(ctx context.Context, client runtimeClient.Client, ccp *eventingv1alpha1.ClusterChannelProvisioner, opts ...ServiceOption) (*corev1.Service, error) {
	svcKey := types.NamespacedName{
		Namespace: system.Namespace(),
		Name:      channelDispatcherServiceName(ccp.Name),
	}
	getSvc := func() (*corev1.Service, error) {
		svc := &corev1.Service{}
		err := client.Get(ctx, svcKey, svc)
		return svc, err
	}
	svc, err := newDispatcherService(ccp, opts...)
	if err != nil {
		return nil, err
	}

	return createK8sService(ctx, client, getSvc, svc)
}

func UpdateClusterChannelProvisionerStatus(ctx context.Context, client runtimeClient.Client, u *eventingv1alpha1.ClusterChannelProvisioner) error {
	o := &eventingv1alpha1.ClusterChannelProvisioner{}
	if err := client.Get(ctx, runtimeClient.ObjectKey{Namespace: u.Namespace, Name: u.Name}, o); err != nil {
		logger := logging.FromContext(ctx)
		logger.Info("Error getting ClusterChannelProvisioner for status update", zap.Error(err), zap.Any("updatedClusterChannelProvisioner", u))
		return err
	}

	if !equality.Semantic.DeepEqual(o.Status, u.Status) {
		o.Status = u.Status
		return client.Status().Update(ctx, o)
	}
	return nil
}

// newDispatcherService creates a new Service for a ClusterChannelProvisioner resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the ClusterChannelProvisioner resource that 'owns' it.
func newDispatcherService(ccp *eventingv1alpha1.ClusterChannelProvisioner, opts ...ServiceOption) (*corev1.Service, error) {
	labels := DispatcherLabels(ccp.Name)
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      channelDispatcherServiceName(ccp.Name),
			Namespace: system.Namespace(),
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(ccp),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					// There is a bug in Istio where named port doesn't work when connecting using an ExternalName service
					// Refer to https://github.com/istio/istio/issues/13193 for more details.
					// TODO: Uncomment Name:"http" when ISTIO fixes the issue
					// Name:       "http",
					Port:       80,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(8080),
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

func DispatcherLabels(ccpName string) map[string]string {
	return map[string]string{
		"clusterChannelProvisioner": ccpName,
		"role":                      "dispatcher",
	}
}

func channelDispatcherServiceName(ccpName string) string {
	return fmt.Sprintf("%s-dispatcher", ccpName)
}
