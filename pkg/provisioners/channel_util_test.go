package provisioners

import (
	"context"
	"errors"
	"fmt"
	"testing"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	controllertesting "github.com/knative/eventing/pkg/controller/testing"

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

	notFound         = k8serrors.NewNotFound(corev1.Resource("any"), "any")
	testInducedError = errors.New("test-induced-error")
)

func init() {
	// Add types to scheme.
	istiov1alpha3.AddToScheme(scheme.Scheme)
	eventingv1alpha1.AddToScheme(scheme.Scheme)
}

func TestChannelUtils(t *testing.T) {
	testCases := []struct {
		name string
		f    func() (metav1.Object, error)
		want metav1.Object
	}{{
		name: "CreateK8sService",
		f: func() (metav1.Object, error) {
			client := fake.NewFakeClient()
			return CreateK8sService(context.TODO(), client, getNewChannel())
		},
		want: makeK8sService(),
	}, {
		name: "CreateK8sService_Existing",
		f: func() (metav1.Object, error) {
			existing := makeK8sService()
			client := fake.NewFakeClient(existing)
			return CreateK8sService(context.TODO(), client, getNewChannel())
		},
		want: makeK8sService(),
	}, {
		name: "CreateVirtualService",
		f: func() (metav1.Object, error) {
			client := fake.NewFakeClient()
			return CreateVirtualService(context.TODO(), client, getNewChannel())
		},
		want: makeVirtualService(),
	}, {
		name: "CreateVirtualService_Existing",
		f: func() (metav1.Object, error) {
			existing := makeVirtualService()
			client := fake.NewFakeClient(existing)
			return CreateVirtualService(context.TODO(), client, getNewChannel())
		},
		want: makeVirtualService(),
	}, {
		name: "CreateVirtualService_ModifiedSpec",
		f: func() (metav1.Object, error) {
			existing := makeVirtualService()
			destHost := fmt.Sprintf("%s-clusterbus.knative-eventing.svc.cluster.local", clusterChannelProvisionerName)
			existing.Spec.Http[0].Route[0].Destination.Host = destHost
			client := fake.NewFakeClient(existing)
			CreateVirtualService(context.TODO(), client, getNewChannel())

			got := &istiov1alpha3.VirtualService{}
			err := client.Get(context.TODO(), runtimeClient.ObjectKey{Namespace: testNS, Name: fmt.Sprintf("%s-channel", channelName)}, got)
			return got, err
		},
		want: makeVirtualService(),
	}, {
		name: "UpdateChannel",
		f: func() (metav1.Object, error) {
			oldChannel := getNewChannel()
			client := fake.NewFakeClient(oldChannel)

			AddFinalizer(oldChannel, "test-finalizer")
			oldChannel.Status.SetAddress("test-domain")
			UpdateChannel(context.TODO(), client, oldChannel)

			got := &eventingv1alpha1.Channel{}
			err := client.Get(context.TODO(), runtimeClient.ObjectKey{Namespace: testNS, Name: channelName}, got)
			return got, err
		},
		want: func() metav1.Object {
			channel := getNewChannel()
			AddFinalizer(channel, "test-finalizer")
			channel.Status.SetAddress("test-domain")
			return channel
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

func TestCreateK8sService(t *testing.T) {
	testCases := map[string]struct {
		get      controllertesting.MockGet
		create   controllertesting.MockCreate
		update   controllertesting.MockUpdate
		expected *corev1.Service
		err      error
	}{
		"error getting svc": {
			get: func(_ runtimeClient.Client, _ context.Context, _ runtimeClient.ObjectKey, _ runtime.Object) (controllertesting.MockHandled, error) {
				return controllertesting.Handled, testInducedError
			},
			err: testInducedError,
		},
		"not found - create error": {
			get: func(_ runtimeClient.Client, _ context.Context, _ runtimeClient.ObjectKey, _ runtime.Object) (controllertesting.MockHandled, error) {
				return controllertesting.Handled, notFound
			},
			create: func(_ runtimeClient.Client, _ context.Context, _ runtime.Object) (controllertesting.MockHandled, error) {
				return controllertesting.Handled, testInducedError
			},
			err: testInducedError,
		},
		"not found - create succeeds": {
			get: func(_ runtimeClient.Client, _ context.Context, _ runtimeClient.ObjectKey, _ runtime.Object) (controllertesting.MockHandled, error) {
				return controllertesting.Handled, notFound
			},
			create: func(_ runtimeClient.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
				svc := obj.(*corev1.Service)
				svc.Spec = makeTamperedK8sService().Spec
				return controllertesting.Handled, nil
			},
			expected: makeTamperedK8sService(),
		},
		"different spec - update fails": {
			get: func(_ runtimeClient.Client, _ context.Context, _ runtimeClient.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
				svc := obj.(*corev1.Service)
				svc.Spec = corev1.ServiceSpec{
					ClusterIP: "set in get",
				}
				return controllertesting.Handled, nil
			},
			update: func(_ runtimeClient.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
				return controllertesting.Handled, testInducedError
			},
			err: testInducedError,
		},
		"different spec - update succeeds": {
			get: func(_ runtimeClient.Client, _ context.Context, _ runtimeClient.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
				svc := obj.(*corev1.Service)
				svc.Spec = corev1.ServiceSpec{
					ClusterIP: "set in get",
				}
				return controllertesting.Handled, nil
			},
			update: func(_ runtimeClient.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
				svc := obj.(*corev1.Service)
				if svc.Spec.ClusterIP != "set in get" {
					return controllertesting.Handled, errors.New("clusterIP should have been overwritten with the version returned by get")
				}
				makeTamperedK8sService().DeepCopyInto(svc)
				return controllertesting.Handled, nil
			},
			expected: makeTamperedK8sService(),
		},
		"found doesn't need altering": {
			get: func(_ runtimeClient.Client, _ context.Context, _ runtimeClient.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
				svc := obj.(*corev1.Service)
				makeK8sService().DeepCopyInto(svc)
				return controllertesting.Handled, nil
			},
			create: func(_ runtimeClient.Client, _ context.Context, _ runtime.Object) (controllertesting.MockHandled, error) {
				return controllertesting.Handled, errors.New("create should not have been called")
			},
			update: func(_ runtimeClient.Client, _ context.Context, _ runtime.Object) (controllertesting.MockHandled, error) {
				return controllertesting.Handled, errors.New("update should not have been called")
			},
			expected: makeK8sService(),
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			client := controllertesting.NewMockClient(fake.NewFakeClient(), controllertesting.Mocks{
				MockGets:    []controllertesting.MockGet{tc.get},
				MockCreates: []controllertesting.MockCreate{tc.create},
				MockUpdates: []controllertesting.MockUpdate{tc.update},
			})
			svc, err := CreateK8sService(context.TODO(), client, getNewChannel())
			if tc.err != err {
				t.Fatalf("Unexpected error. Expected '%s', actual '%v'", tc.err, err)
			}
			if diff := cmp.Diff(tc.expected, svc); diff != "" {
				t.Fatalf("Unexpected service (-want +got): %s", diff)
			}
		})
	}
}

func TestCreateVirtualService(t *testing.T) {
	testCases := map[string]struct {
		get      controllertesting.MockGet
		create   controllertesting.MockCreate
		update   controllertesting.MockUpdate
		expected *istiov1alpha3.VirtualService
		err      error
	}{
		"error getting svc": {
			get: func(_ runtimeClient.Client, _ context.Context, _ runtimeClient.ObjectKey, _ runtime.Object) (controllertesting.MockHandled, error) {
				return controllertesting.Handled, testInducedError
			},
			err: testInducedError,
		},
		"not found - create error": {
			get: func(_ runtimeClient.Client, _ context.Context, _ runtimeClient.ObjectKey, _ runtime.Object) (controllertesting.MockHandled, error) {
				return controllertesting.Handled, notFound
			},
			create: func(_ runtimeClient.Client, _ context.Context, _ runtime.Object) (controllertesting.MockHandled, error) {
				return controllertesting.Handled, testInducedError
			},
			err: testInducedError,
		},
		"not found - create succeeds": {
			get: func(_ runtimeClient.Client, _ context.Context, _ runtimeClient.ObjectKey, _ runtime.Object) (controllertesting.MockHandled, error) {
				return controllertesting.Handled, notFound
			},
			create: func(_ runtimeClient.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
				vs := obj.(*istiov1alpha3.VirtualService)
				vs.Spec = makeTamperedVirtualService().Spec
				return controllertesting.Handled, nil
			},
			expected: makeTamperedVirtualService(),
		},
		"different spec - update fails": {
			get: func(_ runtimeClient.Client, _ context.Context, _ runtimeClient.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
				vs := obj.(*istiov1alpha3.VirtualService)
				vs.Spec = istiov1alpha3.VirtualServiceSpec{
					Gateways: []string{"set in get"},
				}
				return controllertesting.Handled, nil
			},
			update: func(_ runtimeClient.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
				return controllertesting.Handled, testInducedError
			},
			err: testInducedError,
		},
		"different spec - update succeeds": {
			get: func(_ runtimeClient.Client, _ context.Context, _ runtimeClient.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
				vs := obj.(*istiov1alpha3.VirtualService)
				vs.Spec = istiov1alpha3.VirtualServiceSpec{
					Gateways: []string{"set in get"},
				}
				return controllertesting.Handled, nil
			},
			update: func(_ runtimeClient.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
				vs := obj.(*istiov1alpha3.VirtualService)
				makeTamperedVirtualService().DeepCopyInto(vs)
				return controllertesting.Handled, nil
			},
			expected: makeTamperedVirtualService(),
		},
		"found doesn't need altering": {
			get: func(_ runtimeClient.Client, _ context.Context, _ runtimeClient.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
				vs := obj.(*istiov1alpha3.VirtualService)
				makeVirtualService().DeepCopyInto(vs)
				return controllertesting.Handled, nil
			},
			create: func(_ runtimeClient.Client, _ context.Context, _ runtime.Object) (controllertesting.MockHandled, error) {
				return controllertesting.Handled, errors.New("create should not have been called")
			},
			update: func(_ runtimeClient.Client, _ context.Context, _ runtime.Object) (controllertesting.MockHandled, error) {
				return controllertesting.Handled, errors.New("update should not have been called")
			},
			expected: makeVirtualService(),
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			client := controllertesting.NewMockClient(fake.NewFakeClient(), controllertesting.Mocks{
				MockGets:    []controllertesting.MockGet{tc.get},
				MockCreates: []controllertesting.MockCreate{tc.create},
				MockUpdates: []controllertesting.MockUpdate{tc.update},
			})
			svc, err := CreateVirtualService(context.TODO(), client, getNewChannel())
			if tc.err != err {
				t.Fatalf("Unexpected error. Expected '%s', actual '%v'", tc.err, err)
			}
			if diff := cmp.Diff(tc.expected, svc); diff != "" {
				t.Fatalf("Unexpected virtual service (-want +got): %s", diff)
			}
		})
	}
}

func TestAddFinalizer(t *testing.T) {
	testCases := map[string]struct {
		alreadyPresent bool
	}{
		"not present": {
			alreadyPresent: false,
		},
		"already present": {
			alreadyPresent: true,
		},
	}
	finalizer := "test-finalizer"
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			c := getNewChannel()
			if tc.alreadyPresent {
				c.Finalizers = []string{finalizer}
			} else {
				c.Finalizers = []string{}
			}
			addFinalizerResult := AddFinalizer(c, finalizer)
			if tc.alreadyPresent && addFinalizerResult != FinalizerAlreadyPresent {
				t.Errorf("Finalizer already present, expected FinalizerAlreadyPresent. Actual %v", addFinalizerResult)
			} else if !tc.alreadyPresent && addFinalizerResult != FinalizerAdded {
				t.Errorf("Finalizer not already present, expected FinalizerAdded. Actual %v", addFinalizerResult)
			}
		})
	}
}

func TestChannelNames(t *testing.T) {
	testCases := []struct {
		Name string
		F    func() string
		Want string
	}{{
		Name: "ChannelVirtualServiceName",
		F: func() string {
			return ChannelVirtualServiceName("foo")
		},
		Want: "foo-channel",
	}, {
		Name: "ChannelServiceName",
		F: func() string {
			return ChannelServiceName("foo")
		},
		Want: "foo-channel",
	}, {
		Name: "ChannelHostName",
		F: func() string {
			return ChannelHostName("foo", "namespace")
		},
		Want: "foo.namespace.channels.cluster.local",
	}}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			if got := tc.F(); got != tc.Want {
				t.Errorf("want %v, got %v", tc.Want, got)
			}
		})
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

func makeTamperedK8sService() *corev1.Service {
	svc := makeK8sService()
	svc.Spec = corev1.ServiceSpec{
		ClusterIP: "tampered by the unit tests",
	}
	return svc
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
						Host: fmt.Sprintf("%s-dispatcher.knative-eventing.svc.cluster.local", clusterChannelProvisionerName),
						Port: istiov1alpha3.PortSelector{
							Number: PortNumber,
						},
					}},
				}},
			},
		},
	}
}

func makeTamperedVirtualService() *istiov1alpha3.VirtualService {
	vs := makeVirtualService()
	vs.Spec = istiov1alpha3.VirtualServiceSpec{
		Gateways: []string{"tamped by the unit tests"},
	}
	return vs
}
