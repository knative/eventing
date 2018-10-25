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

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/controller"
	"github.com/knative/eventing/pkg/system"
	"github.com/knative/pkg/logging"
)

func CreateDispatcherService(ctx context.Context, client runtimeClient.Client, cp *eventingv1alpha1.ClusterProvisioner) (*corev1.Service, error) {
	svcName := controller.ClusterBusDispatcherServiceName(cp.Name)
	svcKey := types.NamespacedName{
		Namespace: system.Namespace,
		Name:      svcName,
	}
	svc := &corev1.Service{}
	err := client.Get(ctx, svcKey, svc)

	if errors.IsNotFound(err) {
		svc = newDispatcherService(cp)
		err = client.Create(ctx, svc)
	}

	// If an error occurred in either Get or Create, we need to reconcile again.
	if err != nil {
		return nil, err
	}

	return svc, nil
}

func UpdateClusterProvisionerStatus(ctx context.Context, client runtimeClient.Client, u *eventingv1alpha1.ClusterProvisioner) error {
	o := &eventingv1alpha1.ClusterProvisioner{}
	if err := client.Get(ctx, runtimeClient.ObjectKey{Namespace: u.Namespace, Name: u.Name}, o); err != nil {
		logger := logging.FromContext(ctx)
		logger.Info("Error getting ClusterProvisioner for status update", zap.Error(err), zap.Any("updatedClusterProvisioner", u))
		return err
	}

	if !equality.Semantic.DeepEqual(o.Status, u.Status) {
		o.Status = u.Status
		return client.Update(ctx, o)
	}
	return nil
}

// newDispatcherService creates a new Service for a ClusterBus resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the ClusterBus resource that 'owns' it.
func newDispatcherService(cp *eventingv1alpha1.ClusterProvisioner) *corev1.Service {
	labels := DispatcherLabels(cp.Name)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controller.ClusterBusDispatcherServiceName(cp.Name),
			Namespace: system.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cp, schema.GroupVersionKind{
					Group:   eventingv1alpha1.SchemeGroupVersion.Group,
					Version: eventingv1alpha1.SchemeGroupVersion.Version,
					Kind:    "ClusterProvisioner",
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

func DispatcherLabels(cpName string) map[string]string {
	return map[string]string{
		"clusterProvisioner": cpName,
		"role":               "dispatcher",
	}
}
