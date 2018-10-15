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
	"context"
	"fmt"
	"testing"

	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/knative/eventing/pkg/apis/eventing"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/controller/testing"
	"github.com/knative/eventing/pkg/provisioners/kafka/controller"
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

var mockFetchError = controllertesting.Mocks{
	MockGets: []controllertesting.MockGet{
		func(innerClient client.Client, ctx context.Context, key client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
			if _, ok := obj.(*eventingv1alpha1.Channel); ok {
				err := fmt.Errorf("error fetching")
				return controllertesting.Handled, err
			}
			return controllertesting.Unhandled, nil
		},
	},
}

var testCases = []controllertesting.TestCase{
	{
		Name: "new channel with valid provisioner: adds not provisioned status",
		InitialState: []runtime.Object{
			getNewClusterProvisioner(clusterProvisionerName, true),
			getNewChannel(channelName, clusterProvisionerName),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, channelName),
		WantResult:   reconcile.Result{},
		WantPresent: []runtime.Object{
			getNewChannelNotProvisionedStatus(channelName, clusterProvisionerName, "NotImplemented"),
		},
		IgnoreTimes: true,
	},
	{
		Name: "new channel with provisioner not ready: error",
		InitialState: []runtime.Object{
			getNewClusterProvisioner(clusterProvisionerName, false),
			getNewChannel(channelName, clusterProvisionerName),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, channelName),
		WantResult:   reconcile.Result{},
		WantErrMsg:   "ClusterProvisioner " + clusterProvisionerName + " is not ready",
		WantPresent: []runtime.Object{
			getNewChannelNotProvisionedStatus(channelName, clusterProvisionerName,
				"ClusterProvisioner "+clusterProvisionerName+" is not ready"),
		},
		IgnoreTimes: true,
	},
	{
		Name: "new channel with missing provisioner: error",
		InitialState: []runtime.Object{
			getNewChannel(channelName, clusterProvisionerName),
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
			getNewClusterProvisioner("not-our-provisioner", true),
			getNewClusterProvisioner(clusterProvisionerName, true),
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
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, channelName),
		WantResult:   reconcile.Result{},
		WantPresent: []runtime.Object{
			getNewChannelNoProvisioner(channelName),
		},
		IgnoreTimes: true,
	},
	{
		Name:         "channel not found",
		InitialState: []runtime.Object{},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, channelName),
		WantResult:   reconcile.Result{},
		WantPresent:  []runtime.Object{},
		IgnoreTimes:  true,
	},
	{
		Name: "error fetching channel",
		InitialState: []runtime.Object{
			getNewClusterProvisioner(clusterProvisionerName, true),
			getNewChannel(channelName, clusterProvisionerName),
		},
		Mocks:        mockFetchError,
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, channelName),
		WantErrMsg:   "error fetching",
		WantPresent: []runtime.Object{
			getNewClusterProvisioner(clusterProvisionerName, true),
			getNewChannel(channelName, clusterProvisionerName),
		},
	},
}

func TestAllCases(t *testing.T) {
	recorder := record.NewBroadcaster().NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	for _, tc := range testCases {
		c := tc.GetClient()
		r := &reconciler{
			client:   c,
			recorder: recorder,
			log:      log,
			config:   getControllerConfig(),
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

func getNewChannelNotProvisionedStatus(name, provisioner, msg string) *eventingv1alpha1.Channel {
	c := getNewChannel(name, provisioner)
	c.Status = eventingv1alpha1.ChannelStatus{
		Conditions: []duckv1alpha1.Condition{
			{
				Type:    eventingv1alpha1.ChannelConditionProvisioned,
				Status:  corev1.ConditionFalse,
				Reason:  "NotProvisioned",
				Message: msg},
			{
				Type:    eventingv1alpha1.ChannelConditionReady,
				Status:  corev1.ConditionFalse,
				Reason:  "NotProvisioned",
				Message: msg,
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

func getNewClusterProvisioner(name string, isReady bool) *eventingv1alpha1.ClusterProvisioner {
	var condStatus corev1.ConditionStatus
	if isReady {
		condStatus = corev1.ConditionTrue
	} else {
		condStatus = corev1.ConditionFalse
	}
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
					Type:   eventingv1alpha1.ClusterProvisionerConditionReady,
					Status: condStatus,
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

func getControllerConfig() *controller.KafkaProvisionerConfig {
	return &controller.KafkaProvisionerConfig{
		Name:    clusterProvisionerName,
		Brokers: []string{"test-broker"},
	}
}
