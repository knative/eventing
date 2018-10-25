package provisioners

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/knative/eventing/pkg/system"
	"github.com/knative/pkg/apis"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/knative/eventing/pkg/apis/eventing"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
)

const (
	clusterProvisionerName = "kafka"
)

func TestCreateDispatcherService(t *testing.T) {
	want := makeDispatcherService()
	client := fake.NewFakeClient()
	got, _ := CreateDispatcherService(context.TODO(), client, getNewClusterProvisioner())

	ignore := cmpopts.IgnoreTypes(apis.VolatileTime{})
	if diff := cmp.Diff(want, got, ignore); diff != "" {
		t.Errorf("Service (-want, +got) = %v", diff)
	}
}

func TestCreateDispatcherService_Existing(t *testing.T) {
	want := makeDispatcherService()
	client := fake.NewFakeClient(want)
	got, _ := CreateDispatcherService(context.TODO(), client, getNewClusterProvisioner())

	ignore := cmpopts.IgnoreTypes(apis.VolatileTime{})
	if diff := cmp.Diff(want, got, ignore); diff != "" {
		t.Errorf("Service (-want, +got) = %v", diff)
	}
}

func TestUpdateClusterProvisioner(t *testing.T) {
	cp := getNewClusterProvisioner()
	client := fake.NewFakeClient(cp)

	// Update more than just Status
	cp.Status.MarkReady()
	cp.ObjectMeta.Annotations = map[string]string{"test-annotation": "testing"}
	UpdateClusterProvisionerStatus(context.TODO(), client, cp)

	got := &eventingv1alpha1.ClusterProvisioner{}
	client.Get(context.TODO(), runtimeClient.ObjectKey{Namespace: testNS, Name: clusterProvisionerName}, got)

	// Only status should be updated
	want := getNewClusterProvisioner()
	want.Status.MarkReady()

	ignore := cmpopts.IgnoreTypes(apis.VolatileTime{})
	if diff := cmp.Diff(want, got, ignore); diff != "" {
		t.Errorf("ClusterProvisioner (-want, +got) = %v", diff)
	}
}

func getNewClusterProvisioner() *eventingv1alpha1.ClusterProvisioner {
	clusterProvisioner := &eventingv1alpha1.ClusterProvisioner{
		TypeMeta:   ClusterProvisonerType(),
		ObjectMeta: om(testNS, clusterProvisionerName),
		Spec: eventingv1alpha1.ClusterProvisionerSpec{
			Reconciles: metav1.GroupKind{
				Kind:  "Channel",
				Group: eventing.GroupName,
			},
		},
	}
	// selflink is not filled in when we create the object, so clear it
	clusterProvisioner.ObjectMeta.SelfLink = ""
	return clusterProvisioner
}

func ClusterProvisonerType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
		Kind:       "ClusterProvisioner",
	}
}

func makeDispatcherService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: system.Namespace,
			Name:      fmt.Sprintf("%s-clusterbus", clusterProvisionerName),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         eventingv1alpha1.SchemeGroupVersion.String(),
					Kind:               "ClusterProvisioner",
					Name:               clusterProvisionerName,
					Controller:         &truePointer,
					BlockOwnerDeletion: &truePointer,
				},
			},
			Labels: DispatcherLabels(clusterProvisionerName),
		},
		Spec: corev1.ServiceSpec{
			Selector: DispatcherLabels(clusterProvisionerName),
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
