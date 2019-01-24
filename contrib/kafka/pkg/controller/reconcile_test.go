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

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/controller/testing"
	"github.com/knative/eventing/pkg/provisioners"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	clusterChannelProvisionerName = "kafka"
	testNS                        = ""
	otherTestNS                   = "testing"
)

func init() {
	// Add types to scheme
	eventingv1alpha1.AddToScheme(scheme.Scheme)
}

var ClusterChannelProvisionerConditionReady = duckv1alpha1.Condition{
	Type:     eventingv1alpha1.ClusterChannelProvisionerConditionReady,
	Status:   corev1.ConditionTrue,
	Severity: duckv1alpha1.ConditionSeverityError,
}

var mockFetchError = controllertesting.Mocks{
	MockGets: []controllertesting.MockGet{
		func(innerClient client.Client, ctx context.Context, key client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
			if _, ok := obj.(*eventingv1alpha1.ClusterChannelProvisioner); ok {
				err := fmt.Errorf("error fetching")
				return controllertesting.Handled, err
			}
			return controllertesting.Unhandled, nil
		},
	},
}

var testCases = []controllertesting.TestCase{
	{
		Name: "new channel clusterChannelProvisioner: adds status",
		InitialState: []runtime.Object{
			GetNewChannelClusterChannelProvisioner(clusterChannelProvisionerName),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, clusterChannelProvisionerName),
		WantResult:   reconcile.Result{},
		WantPresent: []runtime.Object{
			GetNewChannelClusterChannelProvisionerReady(clusterChannelProvisionerName),
		},
		IgnoreTimes: true,
	},
	{
		Name: "reconciles only associated provisioner",
		InitialState: []runtime.Object{
			GetNewChannelClusterChannelProvisioner("not-default-provisioner"),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, "not-default-provisioner"),
		WantResult:   reconcile.Result{},
		WantPresent: []runtime.Object{
			GetNewChannelClusterChannelProvisioner("not-default-provisioner"),
		},
	},
	{
		Name: "reconciles even when request is namespace-scoped",
		InitialState: []runtime.Object{
			GetNewChannelClusterChannelProvisioner(clusterChannelProvisionerName),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", otherTestNS, clusterChannelProvisionerName),
		WantResult:   reconcile.Result{},
		WantPresent: []runtime.Object{
			GetNewChannelClusterChannelProvisionerReady(clusterChannelProvisionerName),
		},
		IgnoreTimes: true,
	},
	{
		Name:         "clusterChannelProvisioner not found",
		InitialState: []runtime.Object{},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, clusterChannelProvisionerName),
		WantResult:   reconcile.Result{},
		WantPresent:  []runtime.Object{},
	},
	{
		Name: "error fetching clusterChannelProvisioner",
		InitialState: []runtime.Object{
			GetNewChannelClusterChannelProvisioner(clusterChannelProvisionerName),
		},
		Mocks:        mockFetchError,
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, clusterChannelProvisionerName),
		WantErrMsg:   "error fetching",
		WantPresent: []runtime.Object{
			GetNewChannelClusterChannelProvisioner(clusterChannelProvisionerName),
		},
	},
	{
		Name: "deleted clusterChannelProvisioner",
		InitialState: []runtime.Object{
			GetNewChannelClusterChannelProvisionerDeleted(clusterChannelProvisionerName),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, clusterChannelProvisionerName),
		WantResult:   reconcile.Result{},
		WantPresent: []runtime.Object{
			GetNewChannelClusterChannelProvisionerDeleted(clusterChannelProvisionerName),
		},
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
			config:   getControllerConfig(),
		}
		t.Logf("Running test %s", tc.Name)
		t.Run(tc.Name, tc.Runner(t, r, c, recorder))
	}
}

func GetNewChannelClusterChannelProvisioner(name string) *eventingv1alpha1.ClusterChannelProvisioner {
	return getNewClusterChannelProvisioner(name, "Channel")
}

func getNewClusterChannelProvisioner(name string, reconcileKind string) *eventingv1alpha1.ClusterChannelProvisioner {
	clusterChannelProvisioner := &eventingv1alpha1.ClusterChannelProvisioner{
		TypeMeta:   ClusterProvisonerType(),
		ObjectMeta: om(testNS, name),
		Spec:       eventingv1alpha1.ClusterChannelProvisionerSpec{},
	}
	// selflink is not filled in when we create the object, so clear it
	clusterChannelProvisioner.ObjectMeta.SelfLink = ""
	return clusterChannelProvisioner
}

func GetNewChannelClusterChannelProvisionerReady(name string) *eventingv1alpha1.ClusterChannelProvisioner {
	c := GetNewChannelClusterChannelProvisioner(name)
	c.Status = eventingv1alpha1.ClusterChannelProvisionerStatus{
		Conditions: []duckv1alpha1.Condition{
			ClusterChannelProvisionerConditionReady,
		},
	}
	return c
}

func GetNewChannelClusterChannelProvisionerDeleted(name string) *eventingv1alpha1.ClusterChannelProvisioner {
	c := GetNewChannelClusterChannelProvisioner(name)
	deletedTime := metav1.Now().Rfc3339Copy()
	c.DeletionTimestamp = &deletedTime
	return c
}

func ClusterProvisonerType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
		Kind:       "ClusterChannelProvisioner",
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
		Brokers: []string{"test-broker"},
	}
}
