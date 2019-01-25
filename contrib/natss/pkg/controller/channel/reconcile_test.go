/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Veroute.on 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package channel

import (
	"fmt"
	"testing"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/controller/testing"
	"github.com/knative/eventing/pkg/provisioners"
	util "github.com/knative/eventing/pkg/provisioners"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	channelName                   = "test-channel"
	channelNamespace              = "test-namespace"
	clusterChannelProvisionerName = "natss"

	testNS  = "test-namespace"
	testUID = "test-uid"
)

var (
	truePointer = true

	// serviceAddress is the address of the K8s Service. It uses a GeneratedName and the fake client
	// does not fill in Name, so the name is the empty string.
	serviceAddress = fmt.Sprintf("%s.%s.svc.cluster.local", "", testNS)
)

func init() {
	// Add types to scheme
	eventingv1alpha1.AddToScheme(scheme.Scheme)
	istiov1alpha3.AddToScheme(scheme.Scheme)
}

var testCases = []controllertesting.TestCase{
	{
		Name: "new channel with valid provisioner",
		InitialState: []runtime.Object{
			makeNewClusterChannelProvisioner(clusterChannelProvisionerName, true),
			makeNewChannel(channelName, clusterChannelProvisionerName),
			makeVirtualService(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, channelName),
		WantResult:   reconcile.Result{},
		WantPresent: []runtime.Object{
			makeNewChannelProvisionedStatus(channelName, clusterChannelProvisionerName),
		},
		IgnoreTimes: true,
	},
	{
		Name: "new channel with missing provisioner",
		InitialState: []runtime.Object{
			makeNewChannel(channelName, clusterChannelProvisionerName),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, channelName),
		WantResult:   reconcile.Result{},
		WantErrMsg:   "clusterchannelprovisioners.eventing.knative.dev " + "\"" + clusterChannelProvisionerName + "\"" + " not found",
		IgnoreTimes:  true,
	},
	{
		Name: "new channel with provisioner not managed by this controller",
		InitialState: []runtime.Object{
			makeNewChannel(channelName, "not-our-provisioner"),
			makeNewClusterChannelProvisioner("not-our-provisioner", true),
			makeNewClusterChannelProvisioner(clusterChannelProvisionerName, true),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, channelName),
		WantResult:   reconcile.Result{},
		WantPresent: []runtime.Object{
			makeNewChannel(channelName, "not-our-provisioner"),
		},
		IgnoreTimes: true,
	},
	{
		Name: "new channel with provisioner not ready: error",
		InitialState: []runtime.Object{
			makeNewClusterChannelProvisioner(clusterChannelProvisionerName, false),
			makeNewChannel(channelName, clusterChannelProvisionerName),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, channelName),
		WantResult:   reconcile.Result{},
		WantErrMsg:   "ClusterChannelProvisioner " + clusterChannelProvisionerName + " is not ready",
		WantPresent: []runtime.Object{
			makeNewChannelNotProvisionedStatus(channelName, clusterChannelProvisionerName,
				"ClusterChannelProvisioner "+clusterChannelProvisionerName+" is not ready"),
		},
		IgnoreTimes: true,
	},
}

func TestAllCases(t *testing.T) {

	for _, tc := range testCases {
		c := tc.GetClient()
		recorder := tc.GetEventRecorder()
		logger := provisioners.NewProvisionerLoggerFromConfig(provisioners.NewLoggingConfig())
		r := &reconciler{
			client:   c,
			recorder: recorder,
			logger:   logger.Desugar(),
		}
		t.Logf("Running test %s", tc.Name)
		t.Run(tc.Name, tc.Runner(t, r, c, recorder))
	}
}

func TestInjectClient(t *testing.T) {
	r := &reconciler{}
	orig := r.client
	n := fake.NewFakeClient()
	if orig == n {
		t.Errorf("Original and new clients are identical: %v", orig)
	}
	err := r.InjectClient(n)
	if err != nil {
		t.Errorf("Unexpected error injecting the client: %v", err)
	}
	if n != r.client {
		t.Errorf("Unexpected client. Expected: '%v'. Actual: '%v'", n, r.client)
	}
}

func makeNewChannel(name, provisioner string) *eventingv1alpha1.Channel {
	channel := &eventingv1alpha1.Channel{
		TypeMeta:   channelType(),
		ObjectMeta: om(testNS, name),
		Spec: eventingv1alpha1.ChannelSpec{
			Provisioner: &corev1.ObjectReference{
				Name:       provisioner,
				Kind:       "ClusterChannelProvisioner",
				APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
			},
		},
	}
	// selflink is not filled in when we create the object, so clear it
	channel.ObjectMeta.SelfLink = ""
	return channel
}

func makeNewChannelProvisionedStatus(name, provisioner string) *eventingv1alpha1.Channel {
	c := makeNewChannel(name, provisioner)
	c.Status.InitializeConditions()
	c.Status.SetAddress(serviceAddress)
	c.Status.MarkProvisioned()
	return c
}

func makeNewChannelNotProvisionedStatus(name, provisioner, msg string) *eventingv1alpha1.Channel {
	c := makeNewChannel(name, provisioner)
	c.Status.InitializeConditions()
	c.Status.MarkNotProvisioned("NotProvisioned", msg)
	return c
}

func channelType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
		Kind:       "Channel",
	}
}

func makeNewClusterChannelProvisioner(name string, isReady bool) *eventingv1alpha1.ClusterChannelProvisioner {
	var condStatus corev1.ConditionStatus
	if isReady {
		condStatus = corev1.ConditionTrue
	} else {
		condStatus = corev1.ConditionFalse
	}
	clusterChannelProvisioner := &eventingv1alpha1.ClusterChannelProvisioner{
		TypeMeta: metav1.TypeMeta{
			APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
			Kind:       "ClusterChannelProvisioner",
		},
		ObjectMeta: om("", name),
		Spec:       eventingv1alpha1.ClusterChannelProvisionerSpec{},
		Status: eventingv1alpha1.ClusterChannelProvisionerStatus{
			Conditions: []duckv1alpha1.Condition{
				{
					Type:   eventingv1alpha1.ClusterChannelProvisionerConditionReady,
					Status: condStatus,
				},
			},
		},
	}
	// selflink is not filled in when we create the object, so clear it
	clusterChannelProvisioner.ObjectMeta.SelfLink = ""
	return clusterChannelProvisioner
}

func makeVirtualService() *istiov1alpha3.VirtualService {
	return &istiov1alpha3.VirtualService{
		TypeMeta: metav1.TypeMeta{
			APIVersion: istiov1alpha3.SchemeGroupVersion.String(),
			Kind:       "VirtualService",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-channel", testNS),
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
					UID:                testUID,
					Controller:         &truePointer,
					BlockOwnerDeletion: &truePointer,
				},
			},
		},
		Spec: istiov1alpha3.VirtualServiceSpec{
			Hosts: []string{
				serviceAddress,
				fmt.Sprintf("%s.%s.channels.cluster.local", channelName, testNS),
			},
			Http: []istiov1alpha3.HTTPRoute{{
				Rewrite: &istiov1alpha3.HTTPRewrite{
					Authority: fmt.Sprintf("%s.%s.channels.cluster.local", channelName, testNS),
				},
				Route: []istiov1alpha3.DestinationWeight{{
					Destination: istiov1alpha3.Destination{
						Host: "kafka-provisioner.knative-eventing.svc.cluster.local",
						Port: istiov1alpha3.PortSelector{
							Number: util.PortNumber,
						},
					}},
				}},
			},
		},
	}
}

func om(namespace, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
		SelfLink:  fmt.Sprintf("/apis/eventing/v1alpha1/namespaces/%s/object/%s", namespace, name),
	}
}
