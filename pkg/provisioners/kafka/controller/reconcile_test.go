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

package controller

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

	"github.com/knative/eventing/pkg/apis/eventing"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/controller/testing"
	"github.com/knative/eventing/pkg/provisioners"
)

const (
	clusterProvisionerName = "kafka"
	testNS                 = ""
)

func init() {
	// Add types to scheme
	eventingv1alpha1.AddToScheme(scheme.Scheme)
}

var ClusterProvisionerConditionReady = duckv1alpha1.Condition{
	Type:   eventingv1alpha1.ClusterProvisionerConditionReady,
	Status: corev1.ConditionTrue,
}

var mockFetchError = controllertesting.Mocks{
	MockGets: []controllertesting.MockGet{
		func(innerClient client.Client, ctx context.Context, key client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
			if _, ok := obj.(*eventingv1alpha1.ClusterProvisioner); ok {
				err := fmt.Errorf("error fetching")
				return controllertesting.Handled, err
			}
			return controllertesting.Unhandled, nil
		},
	},
}

var testCases = []controllertesting.TestCase{
	{
		Name: "new channel clusterprovisioner: adds status",
		InitialState: []runtime.Object{
			GetNewChannelClusterProvisioner(clusterProvisionerName),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, clusterProvisionerName),
		WantResult:   reconcile.Result{},
		WantPresent: []runtime.Object{
			GetNewChannelClusterProvisionerReady(clusterProvisionerName),
		},
		IgnoreTimes: true,
	},
	{
		Name: "reconciles only channel kind",
		InitialState: []runtime.Object{
			getNewClusterProvisioner(clusterProvisionerName, "Source"),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, clusterProvisionerName),
		WantResult:   reconcile.Result{},
		WantPresent: []runtime.Object{
			getNewClusterProvisioner(clusterProvisionerName, "Source"),
		},
	},
	{
		Name: "reconciles only associated provisioner",
		InitialState: []runtime.Object{
			GetNewChannelClusterProvisioner("not-default-provisioner"),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, "not-default-provisioner"),
		WantResult:   reconcile.Result{},
		WantPresent: []runtime.Object{
			GetNewChannelClusterProvisioner("not-default-provisioner"),
		},
	},
	{
		Name:         "clusterprovisioner not found",
		InitialState: []runtime.Object{},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, clusterProvisionerName),
		WantResult:   reconcile.Result{},
		WantPresent:  []runtime.Object{},
	},
	{
		Name: "error fetching clusterprovisioner",
		InitialState: []runtime.Object{
			GetNewChannelClusterProvisioner(clusterProvisionerName),
		},
		Mocks:        mockFetchError,
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, clusterProvisionerName),
		WantErrMsg:   "error fetching",
		WantPresent: []runtime.Object{
			GetNewChannelClusterProvisioner(clusterProvisionerName),
		},
	},
	{
		Name: "deleted clusterprovisioner",
		InitialState: []runtime.Object{
			GetNewChannelClusterProvisionerDeleted(clusterProvisionerName),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, clusterProvisionerName),
		WantResult:   reconcile.Result{},
		WantPresent: []runtime.Object{
			GetNewChannelClusterProvisionerDeleted(clusterProvisionerName),
		},
	},
}

func TestAllCases(t *testing.T) {
	recorder := record.NewBroadcaster().NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	for _, tc := range testCases {
		c := tc.GetClient()
		logger := provisioners.NewProvisionerLoggerFromConfig(provisioners.NewLoggingConfig())
		r := &reconciler{
			client:   c,
			recorder: recorder,
			logger:   logger.Desugar(),
			config:   getControllerConfig(),
		}
		t.Logf("Running test %s", tc.Name)
		t.Run(tc.Name, tc.Runner(t, r, c))
	}
}

func GetNewChannelClusterProvisioner(name string) *eventingv1alpha1.ClusterProvisioner {
	return getNewClusterProvisioner(name, "Channel")
}

func getNewClusterProvisioner(name string, reconcileKind string) *eventingv1alpha1.ClusterProvisioner {
	clusterProvisioner := &eventingv1alpha1.ClusterProvisioner{
		TypeMeta:   ClusterProvisonerType(),
		ObjectMeta: om(testNS, name),
		Spec: eventingv1alpha1.ClusterProvisionerSpec{
			Reconciles: metav1.GroupKind{
				Kind:  reconcileKind,
				Group: eventing.GroupName,
			},
		},
	}
	// selflink is not filled in when we create the object, so clear it
	clusterProvisioner.ObjectMeta.SelfLink = ""
	return clusterProvisioner
}

func GetNewChannelClusterProvisionerReady(name string) *eventingv1alpha1.ClusterProvisioner {
	c := GetNewChannelClusterProvisioner(name)
	c.Status = eventingv1alpha1.ClusterProvisionerStatus{
		Conditions: []duckv1alpha1.Condition{
			ClusterProvisionerConditionReady,
		},
	}
	return c
}

func GetNewChannelClusterProvisionerDeleted(name string) *eventingv1alpha1.ClusterProvisioner {
	c := GetNewChannelClusterProvisioner(name)
	deletedTime := metav1.Now().Rfc3339Copy()
	c.DeletionTimestamp = &deletedTime
	return c
}

func ClusterProvisonerType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
		Kind:       "ClusterProvisioner",
	}
}

func om(namespace, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
		SelfLink:  fmt.Sprintf("/apis/eventing/v1alpha1/namespaces/%s/object/%s", namespace, name),
	}
}

func getControllerConfig() *KafkaProvisionerConfig {
	return &KafkaProvisionerConfig{
		Name:    clusterProvisionerName,
		Brokers: []string{"test-broker"},
	}
}
