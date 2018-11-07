package provisioners

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/knative/pkg/apis"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
)

const (
	channelName = "test-channel"
	testNS      = "test-namespace"
)

var (
	truePointer = true
)

func init() {
	// Add types to scheme.
	istiov1alpha3.AddToScheme(scheme.Scheme)
	eventingv1alpha1.AddToScheme(scheme.Scheme)
}

func TestCreateK8sService(t *testing.T) {
	want := makeK8sService()
	client := fake.NewFakeClient()
	got, _ := CreateK8sService(context.TODO(), client, getNewChannel())

	ignore := cmpopts.IgnoreTypes(apis.VolatileTime{})
	if diff := cmp.Diff(want, got, ignore); diff != "" {
		t.Errorf("Service (-want, +got) = %v", diff)
	}
}

func TestCreateK8sService_Existing(t *testing.T) {
	want := makeK8sService()
	client := fake.NewFakeClient(want)
	got, _ := CreateK8sService(context.TODO(), client, getNewChannel())

	ignore := cmpopts.IgnoreTypes(apis.VolatileTime{})
	if diff := cmp.Diff(want, got, ignore); diff != "" {
		t.Errorf("Service (-want, +got) = %v", diff)
	}
}

func TestCreateVirtualService(t *testing.T) {
	want := makeVirtualService()
	client := fake.NewFakeClient()
	got, _ := CreateVirtualService(context.TODO(), client, getNewChannel())

	ignore := cmpopts.IgnoreTypes(apis.VolatileTime{})
	if diff := cmp.Diff(want, got, ignore); diff != "" {
		t.Errorf("VirtualService (-want, +got) = %v", diff)
	}
}

func TestCreateVirtualService_Existing(t *testing.T) {
	want := makeVirtualService()
	client := fake.NewFakeClient(want)
	got, _ := CreateVirtualService(context.TODO(), client, getNewChannel())

	ignore := cmpopts.IgnoreTypes(apis.VolatileTime{})
	if diff := cmp.Diff(want, got, ignore); diff != "" {
		t.Errorf("VirtualService (-want, +got) = %v", diff)
	}
}

func TestUpdateChannel(t *testing.T) {
	oldChannel := getNewChannel()
	client := fake.NewFakeClient(oldChannel)

	want := getNewChannel()
	AddFinalizer(want, "test-finalizer")
	want.Status.SetAddress("test-domain")
	UpdateChannel(context.TODO(), client, want)

	got := &eventingv1alpha1.Channel{}
	client.Get(context.TODO(), runtimeClient.ObjectKey{Namespace: testNS, Name: channelName}, got)

	ignore := cmpopts.IgnoreTypes(apis.VolatileTime{})
	if diff := cmp.Diff(want, got, ignore); diff != "" {
		t.Errorf("Channel (-want, +got) = %v", diff)
	}
}

func getNewChannel() *eventingv1alpha1.Channel {
	channel := &eventingv1alpha1.Channel{
		TypeMeta:   channelType(),
		ObjectMeta: om(testNS, channelName),
		Spec: eventingv1alpha1.ChannelSpec{
			Provisioner: &corev1.ObjectReference{
				Name:       clusterChannelProvisionerName,
				Kind:       "ClusterChannelProvisioner",
				APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
			},
		},
	}
	// selflink is not filled in when we create the object, so clear it
	channel.ObjectMeta.SelfLink = ""
	return channel
}

func channelType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
		Kind:       "Channel",
	}
}
func om(namespace, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
		SelfLink:  fmt.Sprintf("/apis/eventing/v1alpha1/namespaces/%s/object/%s", namespace, name),
	}
}

func makeK8sService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-channel", channelName),
			Namespace: testNS,
			Labels: map[string]string{
				"channel":     channelName,
				"provisioner": clusterChannelProvisionerName,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         eventingv1alpha1.SchemeGroupVersion.String(),
					Kind:               "Channel",
					Name:               channelName,
					Controller:         &truePointer,
					BlockOwnerDeletion: &truePointer,
				},
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: PortName,
					Port: PortNumber,
				},
			},
		},
	}
}

func makeVirtualService() *istiov1alpha3.VirtualService {
	return &istiov1alpha3.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-channel", channelName),
			Namespace: testNS,
			Labels: map[string]string{
				"channel":     channelName,
				"provisioner": clusterChannelProvisionerName,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         eventingv1alpha1.SchemeGroupVersion.String(),
					Kind:               "Channel",
					Name:               channelName,
					Controller:         &truePointer,
					BlockOwnerDeletion: &truePointer,
				},
			},
		},
		Spec: istiov1alpha3.VirtualServiceSpec{
			Hosts: []string{
				fmt.Sprintf("%s-channel.%s.svc.cluster.local", channelName, testNS),
				fmt.Sprintf("%s.%s.channels.cluster.local", channelName, testNS),
			},
			Http: []istiov1alpha3.HTTPRoute{{
				Rewrite: &istiov1alpha3.HTTPRewrite{
					Authority: fmt.Sprintf("%s.%s.channels.cluster.local", channelName, testNS),
				},
				Route: []istiov1alpha3.DestinationWeight{{
					Destination: istiov1alpha3.Destination{
						Host: fmt.Sprintf("%s-clusterbus.knative-eventing.svc.cluster.local", clusterChannelProvisionerName),
						Port: istiov1alpha3.PortSelector{
							Number: PortNumber,
						},
					}},
				}},
			},
		},
	}
}
