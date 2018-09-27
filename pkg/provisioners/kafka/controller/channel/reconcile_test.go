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

	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/knative/eventing/pkg/apis/eventing"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/controller/testing"
	"github.com/knative/eventing/pkg/provisioners/kafka/controller"
	"github.com/knative/eventing/pkg/system"
)

var (
	log = logf.Log.WithName("testing")
)

const (
	channelName            = "test-channel"
	clusterProvisionerName = "kafka"
	testNS                 = "test-namespace"
)

func init() {
	// Add types to scheme
	eventingv1alpha1.AddToScheme(scheme.Scheme)
}

var testCases = []controllertesting.TestCase{
	{
		Name: "new channel with valid provisioner: adds not provisioned status",
		InitialState: []runtime.Object{
			getNewClusterProvisioner(clusterProvisionerName),
			getNewChannel(channelName, clusterProvisionerName),
			getControllerConfigMap(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, channelName),
		WantResult:   reconcile.Result{},
		WantPresent: []runtime.Object{
			getNewChannelUnknownStatus(channelName, clusterProvisionerName),
		},
		IgnoreTimes: true,
	},
	{
		Name: "new channel with missing provisioner: error",
		InitialState: []runtime.Object{
			getNewChannel(channelName, clusterProvisionerName),
			getControllerConfigMap(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, channelName),
		WantResult:   reconcile.Result{},
		WantErrMsg:   "clusterprovisioners.eventing.knative.dev \"" + clusterProvisionerName + "\" not found",
		IgnoreTimes:  true,
	},
	{
		Name: "new channel with provisioner not managed by this controller: skips channel",
		InitialState: []runtime.Object{
			getNewChannel(channelName, "not-our-provisioner"),
			getNewClusterProvisioner("not-our-provisioner"),
			getNewClusterProvisioner(clusterProvisionerName),
			getControllerConfigMap(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, channelName),
		WantResult:   reconcile.Result{},
		WantPresent: []runtime.Object{
			getNewChannel(channelName, "not-our-provisioner"),
		},
		IgnoreTimes: true,
	},
	{
		Name: "new channel with missing provisioner reference: skips channel",
		InitialState: []runtime.Object{
			getNewChannelNoProvisioner(channelName),
			getControllerConfigMap(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, channelName),
		WantResult:   reconcile.Result{},
		WantPresent: []runtime.Object{
			getNewChannelNoProvisioner(channelName),
		},
		IgnoreTimes: true,
	},
}

func TestAllCases(t *testing.T) {
	recorder := record.NewBroadcaster().NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	for _, tc := range testCases {
		c := tc.GetClient()
		r := &reconciler{
			client:     c,
			restConfig: &rest.Config{},
			recorder:   recorder,
			log:        log,
		}
		t.Logf("Running test %s", tc.Name)
		t.Run(tc.Name, tc.Runner(t, r, c))
	}
}

func getNewChannelNoProvisioner(name string) *eventingv1alpha1.Channel {
	channel := &eventingv1alpha1.Channel{
		TypeMeta:   channelType(),
		ObjectMeta: om(testNS, name),
		Spec:       eventingv1alpha1.ChannelSpec{},
	}
	// selflink is not filled in when we create the object, so clear it
	channel.ObjectMeta.SelfLink = ""
	return channel
}

func getNewChannel(name, provisioner string) *eventingv1alpha1.Channel {
	channel := &eventingv1alpha1.Channel{
		TypeMeta:   channelType(),
		ObjectMeta: om(testNS, name),
		Spec: eventingv1alpha1.ChannelSpec{
			Provisioner: &eventingv1alpha1.ProvisionerReference{
				Ref: &corev1.ObjectReference{
					Name:       provisioner,
					Kind:       "ClusterProvisioner",
					APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
				},
			},
		},
	}
	// selflink is not filled in when we create the object, so clear it
	channel.ObjectMeta.SelfLink = ""
	return channel
}

func getNewChannelUnknownStatus(name, provisioner string) *eventingv1alpha1.Channel {
	c := getNewChannel(name, provisioner)
	c.Status = eventingv1alpha1.ChannelStatus{
		Conditions: []duckv1alpha1.Condition{
			{
				Type:    eventingv1alpha1.ChannelConditionProvisioned,
				Status:  corev1.ConditionFalse,
				Reason:  "NotProvisioned",
				Message: "NotImplemented"},
			{
				Type:    eventingv1alpha1.ChannelConditionReady,
				Status:  corev1.ConditionFalse,
				Reason:  "NotProvisioned",
				Message: "NotImplemented",
			},
		},
	}
	return c
}

func channelType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
		Kind:       "Channel",
	}
}

func getNewClusterProvisioner(name string) *eventingv1alpha1.ClusterProvisioner {
	clusterProvisioner := &eventingv1alpha1.ClusterProvisioner{
		TypeMeta: metav1.TypeMeta{
			APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
			Kind:       "ClusterProvisioner",
		},
		ObjectMeta: om("", name),
		Spec: eventingv1alpha1.ClusterProvisionerSpec{
			Reconciles: metav1.GroupKind{
				Kind:  "Channel",
				Group: eventing.GroupName,
			},
		},
		Status: eventingv1alpha1.ClusterProvisionerStatus{
			Conditions: []duckv1alpha1.Condition{
				{
					Type:   eventingv1alpha1.ClusterProvisionerConditionProvisionerReady,
					Status: corev1.ConditionTrue,
				},
				{
					Type:   eventingv1alpha1.ClusterProvisionerConditionReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	// selflink is not filled in when we create the object, so clear it
	clusterProvisioner.ObjectMeta.SelfLink = ""
	return clusterProvisioner
}

func om(namespace, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
		SelfLink:  fmt.Sprintf("/apis/eventing/v1alpha1/namespaces/%s/object/%s", namespace, name),
	}
}

func getControllerConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: om(system.Namespace, controller.ControllerConfigMapName),
		Data: map[string]string{
			controller.ProvisionerNameConfigMapKey:      clusterProvisionerName,
			controller.ProvisionerNamespaceConfigMapKey: "",
			controller.BrokerConfigMapKey:               "test-broker",
		},
	}
}
