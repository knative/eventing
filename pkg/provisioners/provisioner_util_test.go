package provisioners

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/knative/pkg/apis"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/system"
)

const (
	clusterChannelProvisionerName = "kafka"
)

func TestCreateDispatcherService(t *testing.T) {
	want := makeDispatcherService()
	client := fake.NewFakeClient()
	got, _ := CreateDispatcherService(context.TODO(), client, getNewClusterChannelProvisioner())

	ignore := cmpopts.IgnoreTypes(apis.VolatileTime{})
	if diff := cmp.Diff(want, got, ignore); diff != "" {
		t.Errorf("Service (-want, +got) = %v", diff)
	}
}

func TestCreateDispatcherService_Existing(t *testing.T) {
	want := makeDispatcherService()
	client := fake.NewFakeClient(want)
	got, _ := CreateDispatcherService(context.TODO(), client, getNewClusterChannelProvisioner())

	ignore := cmpopts.IgnoreTypes(apis.VolatileTime{})
	if diff := cmp.Diff(want, got, ignore); diff != "" {
		t.Errorf("Service (-want, +got) = %v", diff)
	}
}

func TestUpdateClusterChannelProvisioner(t *testing.T) {
	ccp := getNewClusterChannelProvisioner()
	client := fake.NewFakeClient(ccp)

	// Update more than just Status
	ccp.Status.MarkReady()
	ccp.ObjectMeta.Annotations = map[string]string{"test-annotation": "testing"}
	UpdateClusterChannelProvisionerStatus(context.TODO(), client, ccp)

	got := &eventingv1alpha1.ClusterChannelProvisioner{}
	client.Get(context.TODO(), runtimeClient.ObjectKey{Namespace: testNS, Name: clusterChannelProvisionerName}, got)

	// Only status should be updated
	want := getNewClusterChannelProvisioner()
	want.Status.MarkReady()

	ignore := cmpopts.IgnoreTypes(apis.VolatileTime{})
	if diff := cmp.Diff(want, got, ignore); diff != "" {
		t.Errorf("ClusterChannelProvisioner (-want, +got) = %v", diff)
	}
}

func getNewClusterChannelProvisioner() *eventingv1alpha1.ClusterChannelProvisioner {
	clusterChannelProvisioner := &eventingv1alpha1.ClusterChannelProvisioner{
		TypeMeta:   ClusterChannelProvisionerType(),
		ObjectMeta: om(testNS, clusterChannelProvisionerName),
		Spec:       eventingv1alpha1.ClusterChannelProvisionerSpec{},
	}
	// selflink is not filled in when we create the object, so clear it
	clusterChannelProvisioner.ObjectMeta.SelfLink = ""
	return clusterChannelProvisioner
}

func ClusterChannelProvisionerType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
		Kind:       "ClusterChannelProvisioner",
	}
}

func makeDispatcherService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: system.Namespace,
			Name:      fmt.Sprintf("%s-clusterbus", clusterChannelProvisionerName),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         eventingv1alpha1.SchemeGroupVersion.String(),
					Kind:               "ClusterChannelProvisioner",
					Name:               clusterChannelProvisionerName,
					Controller:         &truePointer,
					BlockOwnerDeletion: &truePointer,
				},
			},
			Labels: DispatcherLabels(clusterChannelProvisionerName),
		},
		Spec: corev1.ServiceSpec{
			Selector: DispatcherLabels(clusterChannelProvisionerName),
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
