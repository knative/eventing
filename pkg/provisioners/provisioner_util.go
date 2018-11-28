package provisioners

import (
	"context"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	"fmt"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/system"
	"github.com/knative/pkg/logging"
)

func CreateDispatcherService(ctx context.Context, client runtimeClient.Client, ccp *eventingv1alpha1.ClusterChannelProvisioner) (*corev1.Service, error) {
	svcName := ChannelDispatcherServiceName(ccp.Name)
	svcKey := types.NamespacedName{
		Namespace: system.Namespace,
		Name:      svcName,
	}
	svc := &corev1.Service{}
	err := client.Get(ctx, svcKey, svc)

	if errors.IsNotFound(err) {
		svc = newDispatcherService(ccp)
		err = client.Create(ctx, svc)
		if err != nil {
			return nil, err
		}
		return svc, nil
	}
	if err != nil {
		return nil, err
	}

	expected := newDispatcherService(ccp)
	// spec.clusterIP is immutable and is set on existing services. If we don't set this
	// to the same value, we will encounter an error while updating.
	expected.Spec.ClusterIP = svc.Spec.ClusterIP
	if !equality.Semantic.DeepDerivative(expected.Spec, svc.Spec) {
		svc.Spec = expected.Spec
		err := client.Update(ctx, svc)
		if err != nil {
			return nil, err
		}
	}
	return svc, nil
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
		return client.Update(ctx, o)
	}
	return nil
}

// newDispatcherService creates a new Service for a ClusterChannelProvisioner resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the ClusterChannelProvisioner resource that 'owns' it.
func newDispatcherService(ccp *eventingv1alpha1.ClusterChannelProvisioner) *corev1.Service {
	labels := DispatcherLabels(ccp.Name)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ChannelDispatcherServiceName(ccp.Name),
			Namespace: system.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(ccp, schema.GroupVersionKind{
					Group:   eventingv1alpha1.SchemeGroupVersion.Group,
					Version: eventingv1alpha1.SchemeGroupVersion.Version,
					Kind:    "ClusterChannelProvisioner",
				}),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}
}

func DispatcherLabels(ccpName string) map[string]string {
	return map[string]string{
		"clusterChannelProvisioner": ccpName,
		"role":                      "dispatcher",
	}
}

func ChannelDispatcherServiceName(ccpName string) string {
	return fmt.Sprintf("%s-dispatcher", ccpName)
}
