package provisioners

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/types"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"knative.dev/pkg/apis"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing"
)

const (
	clusterChannelProvisionerName      = "kafka"
	otherClusterChannelProvisionerName = "kafka-new"
	testClusterIP                      = "10.59.249.3"

	ccpUID = types.UID("test-ccp-uid")
)

func TestProvisionerUtils(t *testing.T) {
	testCases := []struct {
		name string
		f    func() (metav1.Object, error)
		want metav1.Object
	}{{
		name: "CreateDispatcherService",
		f: func() (metav1.Object, error) {
			client := fake.NewFakeClient()
			return CreateDispatcherService(context.TODO(), client, getNewClusterChannelProvisioner())
		},
		want: makeDispatcherService(),
	}, {
		name: "CreateDispatcherService_Existing",
		f: func() (metav1.Object, error) {
			existing := makeDispatcherService()
			client := fake.NewFakeClient(existing)
			return CreateDispatcherService(context.TODO(), client, getNewClusterChannelProvisioner())
		},
		want: makeDispatcherService(),
	}, {
		name: "CreateDispatcherService_ModifiedSpec",
		f: func() (metav1.Object, error) {
			existing := makeDispatcherService()
			existing.Spec.Selector = map[string]string{
				"clusterChannelProvisioner": otherClusterChannelProvisionerName,
				"role":                      "dispatcher",
			}
			client := fake.NewFakeClient(existing)
			CreateDispatcherService(context.TODO(), client, getNewClusterChannelProvisioner())

			got := &corev1.Service{}
			err := client.Get(context.TODO(), runtimeClient.ObjectKey{Namespace: system.Namespace(), Name: fmt.Sprintf("%s-dispatcher", clusterChannelProvisionerName)}, got)
			return got, err
		},
		want: makeDispatcherService(),
	}, {
		name: "CreateDispatcherService_DoNotModifyClusterIP",
		f: func() (metav1.Object, error) {
			existing := makeDispatcherService()
			existing.Spec.ClusterIP = testClusterIP
			client := fake.NewFakeClient(existing)
			CreateDispatcherService(context.TODO(), client, getNewClusterChannelProvisioner())

			got := &corev1.Service{}
			err := client.Get(context.TODO(), runtimeClient.ObjectKey{Namespace: system.Namespace(), Name: fmt.Sprintf("%s-dispatcher", clusterChannelProvisionerName)}, got)
			return got, err
		},
		want: func() metav1.Object {
			svc := makeDispatcherService()
			svc.Spec.ClusterIP = testClusterIP
			return svc
		}(),
	}, {
		name: "UpdateClusterChannelProvisioner",
		f: func() (metav1.Object, error) {
			ccp := getNewClusterChannelProvisioner()
			client := fake.NewFakeClient(ccp)

			// Update more than just Status
			ccp.Status.MarkReady()
			ccp.ObjectMeta.Annotations = map[string]string{"test-annotation": "testing"}
			UpdateClusterChannelProvisionerStatus(context.TODO(), client, ccp)

			got := &eventingv1alpha1.ClusterChannelProvisioner{}
			err := client.Get(context.TODO(), runtimeClient.ObjectKey{Namespace: testNS, Name: clusterChannelProvisionerName}, got)
			return got, err
		},
		want: func() metav1.Object {
			// Only status should be updated
			ccp := getNewClusterChannelProvisioner()
			ccp.Status.MarkReady()
			return ccp
		}(),
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ignore := cmpopts.IgnoreTypes(apis.VolatileTime{})
			got, err := tc.f()
			if err != nil {
				t.Errorf("Unexpected error %+v", err)
			}
			if diff := cmp.Diff(tc.want, got, ignore); diff != "" {
				t.Errorf("%s (-want, +got) = %v", tc.name, diff)
			}
		})
	}
}

func TestNames(t *testing.T) {
	testCases := []struct {
		Name string
		F    func() string
		Want string
	}{{
		Name: "channelDispatcherServiceName",
		F: func() string {
			return channelDispatcherServiceName("foo")
		},
		Want: "foo-dispatcher",
	}}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			if got := tc.F(); got != tc.Want {
				t.Errorf("want %v, got %v", tc.Want, got)
			}
		})
	}
}

func getNewClusterChannelProvisioner() *eventingv1alpha1.ClusterChannelProvisioner {
	clusterChannelProvisioner := &eventingv1alpha1.ClusterChannelProvisioner{
		TypeMeta:   ClusterChannelProvisionerType(),
		ObjectMeta: om(testNS, clusterChannelProvisionerName, ccpUID),
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
			Namespace: system.Namespace(),
			Name:      fmt.Sprintf("%s-dispatcher", clusterChannelProvisionerName),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         eventingv1alpha1.SchemeGroupVersion.String(),
					Kind:               "ClusterChannelProvisioner",
					Name:               clusterChannelProvisionerName,
					UID:                ccpUID,
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
					Port:       80,
					TargetPort: intstr.FromInt(8080),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}
