package provisioners

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/types"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	controllertesting "github.com/knative/eventing/pkg/reconciler/testing"

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
	"github.com/knative/eventing/pkg/utils"
	_ "github.com/knative/pkg/system/testing"
)

const (
	channelName = "test-channel"
	channelUID  = types.UID("test-channel-uid")
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
			return CreateVirtualService(context.TODO(), client, getNewChannel(), makeK8sService())
		},
		want: makeVirtualService(),
	}, {
		name: "CreateVirtualService_Existing",
		f: func() (metav1.Object, error) {
			existing := makeVirtualService()
			client := fake.NewFakeClient(existing)
			return CreateVirtualService(context.TODO(), client, getNewChannel(), makeK8sService())
		},
		want: makeVirtualService(),
	}, {
		name: "CreateVirtualService_ModifiedSpec",
		f: func() (metav1.Object, error) {
			existing := makeVirtualService()
			destHost := fmt.Sprintf("%s-clusterbus.knative-eventing.svc.%s", clusterChannelProvisionerName, utils.GetClusterDomainName())
			existing.Spec.Http[0].Route[0].Destination.Host = destHost
			client := fake.NewFakeClient(existing)
			CreateVirtualService(context.TODO(), client, getNewChannel(), makeK8sService())

			got := &istiov1alpha3.VirtualService{}
			got, err := getVirtualService(context.TODO(), client, getNewChannel())
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
		list     controllertesting.MockList
		create   controllertesting.MockCreate
		update   controllertesting.MockUpdate
		expected *corev1.Service
		err      error
	}{
		"error getting svc": {
			list: func(_ runtimeClient.Client, _ context.Context, _ *runtimeClient.ListOptions, _ runtime.Object) (controllertesting.MockHandled, error) {
				return controllertesting.Handled, testInducedError
			},
			err: testInducedError,
		},
		"not found - create error": {
			create: func(_ runtimeClient.Client, _ context.Context, _ runtime.Object) (controllertesting.MockHandled, error) {
				return controllertesting.Handled, testInducedError
			},
			err: testInducedError,
		},
		"not found - create succeeds": {
			create: func(_ runtimeClient.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
				svc := obj.(*corev1.Service)
				svc.Spec = makeTamperedK8sService().Spec
				return controllertesting.Handled, nil
			},
			expected: makeTamperedK8sService(),
		},
		"different spec - update fails": {
			list: func(_ runtimeClient.Client, _ context.Context, _ *runtimeClient.ListOptions, obj runtime.Object) (controllertesting.MockHandled, error) {
				l := obj.(*corev1.ServiceList)
				l.Items = []corev1.Service{
					{
						ObjectMeta: metav1.ObjectMeta{
							OwnerReferences: []metav1.OwnerReference{
								{
									Controller: &truePointer,
									UID:        channelUID,
								},
							},
						},
						Spec: corev1.ServiceSpec{
							ClusterIP: "set in get",
						},
					},
				}
				return controllertesting.Handled, nil
			},
			update: func(_ runtimeClient.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
				return controllertesting.Handled, testInducedError
			},
			err: testInducedError,
		},
		"different spec - update succeeds": {
			list: func(_ runtimeClient.Client, _ context.Context, _ *runtimeClient.ListOptions, obj runtime.Object) (controllertesting.MockHandled, error) {
				l := obj.(*corev1.ServiceList)
				l.Items = []corev1.Service{
					{
						ObjectMeta: metav1.ObjectMeta{
							OwnerReferences: []metav1.OwnerReference{
								{
									Controller: &truePointer,
									UID:        channelUID,
								},
							},
						},
						Spec: corev1.ServiceSpec{
							ClusterIP: "set in get",
						},
					},
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
			list: func(_ runtimeClient.Client, _ context.Context, _ *runtimeClient.ListOptions, obj runtime.Object) (controllertesting.MockHandled, error) {
				l := obj.(*corev1.ServiceList)
				l.Items = []corev1.Service{*makeK8sService()}
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
			mocks := controllertesting.Mocks{}
			if tc.list != nil {
				mocks.MockLists = []controllertesting.MockList{tc.list}
			}
			if tc.create != nil {
				mocks.MockCreates = []controllertesting.MockCreate{tc.create}
			}
			if tc.update != nil {
				mocks.MockUpdates = []controllertesting.MockUpdate{tc.update}
			}
			client := controllertesting.NewMockClient(fake.NewFakeClient(), mocks)
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
		list     controllertesting.MockList
		create   controllertesting.MockCreate
		update   controllertesting.MockUpdate
		expected *istiov1alpha3.VirtualService
		err      error
	}{
		"error getting svc": {
			list: func(_ runtimeClient.Client, _ context.Context, _ *runtimeClient.ListOptions, _ runtime.Object) (controllertesting.MockHandled, error) {
				return controllertesting.Handled, testInducedError
			},
			err: testInducedError,
		},
		"not found - create error": {
			create: func(_ runtimeClient.Client, _ context.Context, _ runtime.Object) (controllertesting.MockHandled, error) {
				return controllertesting.Handled, testInducedError
			},
			err: testInducedError,
		},
		"not found - create succeeds": {
			create: func(_ runtimeClient.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
				vs := obj.(*istiov1alpha3.VirtualService)
				vs.Spec = makeTamperedVirtualService().Spec
				return controllertesting.Handled, nil
			},
			expected: makeTamperedVirtualService(),
		},
		"different spec - update fails": {
			list: func(_ runtimeClient.Client, _ context.Context, _ *runtimeClient.ListOptions, obj runtime.Object) (controllertesting.MockHandled, error) {
				l := obj.(*istiov1alpha3.VirtualServiceList)
				l.Items = []istiov1alpha3.VirtualService{
					{
						ObjectMeta: metav1.ObjectMeta{
							OwnerReferences: []metav1.OwnerReference{
								{
									Controller: &truePointer,
									UID:        channelUID,
								},
							},
						},
						Spec: istiov1alpha3.VirtualServiceSpec{
							Gateways: []string{"set in get"},
						},
					},
				}
				return controllertesting.Handled, nil
			},
			update: func(_ runtimeClient.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
				return controllertesting.Handled, testInducedError
			},
			err: testInducedError,
		},
		"different spec - update succeeds": {
			list: func(_ runtimeClient.Client, _ context.Context, _ *runtimeClient.ListOptions, obj runtime.Object) (controllertesting.MockHandled, error) {
				l := obj.(*istiov1alpha3.VirtualServiceList)
				l.Items = []istiov1alpha3.VirtualService{
					{
						ObjectMeta: metav1.ObjectMeta{
							OwnerReferences: []metav1.OwnerReference{
								{
									Controller: &truePointer,
									UID:        channelUID,
								},
							},
						},
						Spec: istiov1alpha3.VirtualServiceSpec{
							Gateways: []string{"set in get"},
						},
					},
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
			list: func(_ runtimeClient.Client, _ context.Context, _ *runtimeClient.ListOptions, obj runtime.Object) (controllertesting.MockHandled, error) {
				l := obj.(*istiov1alpha3.VirtualServiceList)
				l.Items = []istiov1alpha3.VirtualService{*makeVirtualService()}
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
			mocks := controllertesting.Mocks{}
			if tc.list != nil {
				mocks.MockLists = []controllertesting.MockList{tc.list}
			}
			if tc.create != nil {
				mocks.MockCreates = []controllertesting.MockCreate{tc.create}
			}
			if tc.update != nil {
				mocks.MockUpdates = []controllertesting.MockUpdate{tc.update}
			}
			client := controllertesting.NewMockClient(fake.NewFakeClient(), mocks)
			vs, err := CreateVirtualService(context.TODO(), client, getNewChannel(), makeK8sService())
			if tc.err != err {
				t.Fatalf("Unexpected error. Expected '%s', actual '%v'", tc.err, err)
			}
			if diff := cmp.Diff(tc.expected, vs); diff != "" {
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
		Name: "channelVirtualServiceName",
		F: func() string {
			return channelVirtualServiceName("foo")
		},
		Want: "foo-channel-",
	}, {
		Name: "channelServiceName",
		F: func() string {
			return channelServiceName("foo")
		},
		Want: "foo-channel-",
	}, {
		Name: "channelHostName",
		F: func() string {
			return channelHostName("foo", "namespace")
		},
		Want: "foo.namespace.channels." + utils.GetClusterDomainName(),
	}}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			if got := tc.F(); got != tc.Want {
				t.Errorf("want %v, got %v", tc.Want, got)
			}
		})
	}
}

func TestExpectedLabelsPresent(t *testing.T) {
	tests := []struct {
		name       string
		actual     map[string]string
		expected   map[string]string
		shouldFail bool
	}{
		{
			name:   "actual nil",
			actual: nil,
			expected: map[string]string{
				EventingChannelLabel:        "channel-1",
				OldEventingChannelLabel:     "channel-1",
				EventingProvisionerLabel:    "provisioner-1",
				OldEventingProvisionerLabel: "provisioner-1",
			},
			shouldFail: true,
		},
		{
			name: "missing some labels",
			actual: map[string]string{
				OldEventingChannelLabel:     "channel-1",
				OldEventingProvisionerLabel: "provisioner-1",
			},
			expected: map[string]string{
				EventingChannelLabel:        "channel-1",
				OldEventingChannelLabel:     "channel-1",
				EventingProvisionerLabel:    "provisioner-1",
				OldEventingProvisionerLabel: "provisioner-1",
			},
			shouldFail: true,
		},
		{
			name: "all labels but mismatched value",
			actual: map[string]string{
				EventingChannelLabel:        "channel-1",
				OldEventingChannelLabel:     "channel-1",
				EventingProvisionerLabel:    "provisioner",
				OldEventingProvisionerLabel: "provisioner-1",
			},
			expected: map[string]string{
				EventingChannelLabel:        "channel-1",
				OldEventingChannelLabel:     "channel-1",
				EventingProvisionerLabel:    "provisioner-1",
				OldEventingProvisionerLabel: "provisioner-1",
			},
			shouldFail: true,
		},
		{
			name: "all good",
			actual: map[string]string{
				EventingChannelLabel:        "channel-1",
				OldEventingChannelLabel:     "channel-1",
				EventingProvisionerLabel:    "provisioner-1",
				OldEventingProvisionerLabel: "provisioner-1",
			},
			expected: map[string]string{
				EventingChannelLabel:        "channel-1",
				OldEventingChannelLabel:     "channel-1",
				EventingProvisionerLabel:    "provisioner-1",
				OldEventingProvisionerLabel: "provisioner-1",
			},
			shouldFail: false,
		},
		{
			name: "all good but with extra label",
			actual: map[string]string{
				EventingChannelLabel:                 "channel-1",
				OldEventingChannelLabel:              "channel-1",
				EventingProvisionerLabel:             "provisioner-1",
				OldEventingProvisionerLabel:          "provisioner-1",
				"extra-label-that-should-be-ignored": "foo",
			},
			expected: map[string]string{
				EventingChannelLabel:        "channel-1",
				OldEventingChannelLabel:     "channel-1",
				EventingProvisionerLabel:    "provisioner-1",
				OldEventingProvisionerLabel: "provisioner-1",
			},
			shouldFail: false,
		},
	}

	for _, test := range tests {
		result := expectedLabelsPresent(test.actual, test.expected)
		if result && test.shouldFail {
			t.Errorf("Test: %s supposed to %v but %v", test.name, func(v bool) string {
				if v {
					return "fail"
				}
				return "succeed"
			}(test.shouldFail), func(v bool) string {
				if v {
					return "fail"
				}
				return "succeed"
			}(result))
		}
	}
}

func getNewChannel() *eventingv1alpha1.Channel {
	channel := &eventingv1alpha1.Channel{
		TypeMeta:   channelType(),
		ObjectMeta: om(testNS, channelName, channelUID),
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

func om(namespace, name string, uid types.UID) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
		UID:       uid,
		SelfLink:  fmt.Sprintf("/apis/eventing/v1alpha1/namespaces/%s/object/%s", namespace, name),
	}
}

func makeK8sService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-channel-", channelName),
			Namespace:    testNS,
			Labels: map[string]string{
				EventingChannelLabel:        channelName,
				OldEventingChannelLabel:     channelName,
				EventingProvisionerLabel:    clusterChannelProvisionerName,
				OldEventingProvisionerLabel: clusterChannelProvisionerName,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         eventingv1alpha1.SchemeGroupVersion.String(),
					Kind:               "Channel",
					Name:               channelName,
					UID:                channelUID,
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
			GenerateName: fmt.Sprintf("%s-channel-", channelName),
			Namespace:    testNS,
			Labels: map[string]string{
				EventingChannelLabel:        channelName,
				OldEventingChannelLabel:     channelName,
				EventingProvisionerLabel:    clusterChannelProvisionerName,
				OldEventingProvisionerLabel: clusterChannelProvisionerName,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         eventingv1alpha1.SchemeGroupVersion.String(),
					Kind:               "Channel",
					Name:               channelName,
					UID:                channelUID,
					Controller:         &truePointer,
					BlockOwnerDeletion: &truePointer,
				},
			},
		},
		Spec: istiov1alpha3.VirtualServiceSpec{
			Hosts: []string{
				// The fake client doesn't fill in a Name when GeneratedName is used, so the
				// Channel's Name will be the empty string.
				fmt.Sprintf("%s.%s.svc.%s", "", testNS, utils.GetClusterDomainName()),
				fmt.Sprintf("%s.%s.channels.%s", channelName, testNS, utils.GetClusterDomainName()),
			},
			Http: []istiov1alpha3.HTTPRoute{{
				Rewrite: &istiov1alpha3.HTTPRewrite{
					Authority: fmt.Sprintf("%s.%s.channels.%s", channelName, testNS, utils.GetClusterDomainName()),
				},
				Route: []istiov1alpha3.DestinationWeight{{
					Destination: istiov1alpha3.Destination{
						Host: fmt.Sprintf("%s-dispatcher.knative-testing.svc.%s", clusterChannelProvisionerName, utils.GetClusterDomainName()),
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
