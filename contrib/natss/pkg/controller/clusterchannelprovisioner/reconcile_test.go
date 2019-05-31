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

package clusterchannelprovisioner

import (
	"context"
	"fmt"
	"testing"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners"
	controllertesting "github.com/knative/eventing/pkg/reconciler/testing"
	"github.com/knative/pkg/apis"
	"github.com/knative/pkg/system"
	_ "github.com/knative/pkg/system/testing"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	clusterChannelProvisionerName = "natss"
	testNS                        = ""
	otherTestNS                   = "testing"
)

func init() {
	// Add types to scheme
	eventingv1alpha1.AddToScheme(scheme.Scheme)
}

var (
	truePointer = true
	deletedTime = metav1.Now().Rfc3339Copy()
)

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
			makeClusterChannelProvisioner(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, clusterChannelProvisionerName),
		WantResult:   reconcile.Result{},
		WantPresent: []runtime.Object{
			makeReadyClusterChannelProvisioner(),
		},
		IgnoreTimes: true,
	},
	{
		Name: "reconciles only associated provisioner",
		InitialState: []runtime.Object{
			getClusterChannelProvisioner("not-default-provisioner"),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, "not-default-provisioner"),
		WantResult:   reconcile.Result{},
		WantPresent: []runtime.Object{
			getClusterChannelProvisioner("not-default-provisioner"),
		},
	},
	{
		Name: "reconciles even when request is namespace-scoped",
		InitialState: []runtime.Object{
			makeClusterChannelProvisioner(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", otherTestNS, clusterChannelProvisionerName),
		WantResult:   reconcile.Result{},
		WantPresent: []runtime.Object{
			makeReadyClusterChannelProvisioner(),
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
			makeClusterChannelProvisioner(),
		},
		Mocks:        mockFetchError,
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, clusterChannelProvisionerName),
		WantErrMsg:   "error fetching",
		WantPresent: []runtime.Object{
			makeClusterChannelProvisioner(),
		},
	},
	{
		Name: "deleted clusterChannelProvisioner",
		InitialState: []runtime.Object{
			makeDeletedClusterChannelProvisioner(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, clusterChannelProvisionerName),
		WantResult:   reconcile.Result{},
		WantPresent: []runtime.Object{
			makeDeletedClusterChannelProvisioner(),
		},
	},
	{
		Name: "Create dispatcher - not owned by CCP",
		InitialState: []runtime.Object{
			getClusterChannelProvisioner(clusterChannelProvisionerName),
			makeK8sServiceNotOwnedByClusterChannelProvisioner(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, clusterChannelProvisionerName),
		WantPresent: []runtime.Object{
			makeReadyClusterChannelProvisioner(),
		},
	},
	{
		Name: "Create dispatcher succeeds",
		InitialState: []runtime.Object{
			getClusterChannelProvisioner(clusterChannelProvisionerName),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, clusterChannelProvisionerName),
		WantPresent: []runtime.Object{
			makeReadyClusterChannelProvisioner(),
			makeK8sService(),
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
		}
		tc.IgnoreTimes = true
		t.Logf("Running test %s", tc.Name)
		t.Run(tc.Name, tc.Runner(t, r, c, recorder))
	}
}

func getClusterChannelProvisioner(name string) *eventingv1alpha1.ClusterChannelProvisioner {
	return getNewClusterChannelProvisioner(name, "Channel")
}

func getNewClusterChannelProvisioner(name string, reconcileKind string) *eventingv1alpha1.ClusterChannelProvisioner {
	clusterChannelProvisioner := &eventingv1alpha1.ClusterChannelProvisioner{
		TypeMeta:   clusterProvisonerType(),
		ObjectMeta: om(testNS, name),
		Spec:       eventingv1alpha1.ClusterChannelProvisionerSpec{},
	}
	// selflink is not filled in when we create the object, so clear it
	clusterChannelProvisioner.ObjectMeta.SelfLink = ""
	return clusterChannelProvisioner
}

func makeDeletedClusterChannelProvisioner() *eventingv1alpha1.ClusterChannelProvisioner {
	c := makeClusterChannelProvisioner()
	c.DeletionTimestamp = &deletedTime
	return c
}

func clusterProvisonerType() metav1.TypeMeta {
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

func makeClusterChannelProvisioner() *eventingv1alpha1.ClusterChannelProvisioner {
	return &eventingv1alpha1.ClusterChannelProvisioner{
		TypeMeta: metav1.TypeMeta{
			APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
			Kind:       "ClusterChannelProvisioner",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: Name,
		},
		Spec: eventingv1alpha1.ClusterChannelProvisionerSpec{},
	}
}

func makeReadyClusterChannelProvisioner() *eventingv1alpha1.ClusterChannelProvisioner {
	ccp := makeClusterChannelProvisioner()
	ccp.Status.Conditions = []apis.Condition{{
		Type:     apis.ConditionReady,
		Status:   corev1.ConditionTrue,
		Severity: apis.ConditionSeverityError,
	}}
	return ccp
}

func makeK8sService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: system.Namespace(),
			Name:      fmt.Sprintf("%s-dispatcher", Name),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         eventingv1alpha1.SchemeGroupVersion.String(),
					Kind:               "ClusterChannelProvisioner",
					Name:               Name,
					Controller:         &truePointer,
					BlockOwnerDeletion: &truePointer,
				},
			},
			Labels: provisioners.DispatcherLabels(Name),
		},
		Spec: corev1.ServiceSpec{
			Selector: provisioners.DispatcherLabels(Name),
			Ports: []corev1.ServicePort{
				{
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}
}

func makeK8sServiceNotOwnedByClusterChannelProvisioner() *corev1.Service {
	svc := makeK8sService()
	svc.OwnerReferences = nil
	return svc
}
