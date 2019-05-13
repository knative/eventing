/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package inmemorychannel

import (
	//	"fmt"
	//	"net/url"
	"testing"

	"github.com/knative/eventing/pkg/apis/messaging/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	//	listers "github.com/knative/eventing/pkg/client/listers/messaging/v1alpha1"
	"github.com/knative/eventing/pkg/reconciler"
	reconciletesting "github.com/knative/eventing/pkg/reconciler/testing"
	//	"github.com/knative/eventing/pkg/utils"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	logtesting "github.com/knative/pkg/logging/testing"
	. "github.com/knative/pkg/reconciler/testing"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	//	appsv1listers "k8s.io/client-go/listers/apps/v1"
	//	corev1listers "k8s.io/client-go/listers/core/v1"
	//	clientgotesting "k8s.io/client-go/testing"
)

const (
	testNS                   = "test-namespace"
	imcName                  = "test-imc"
	dispatcherDeploymentName = "test-deployment"
	dispatcherServiceName    = "test-service"

	subscriberAPIVersion = "v1"
	subscriberKind       = "Service"
	subscriberName       = "subscriberName"
	subscriberURI        = "http://example.com/subscriber"
)

var (
	trueVal = true
	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()

	// Map of events to set test cases' expectations easier.
/*
	events = map[string]corev1.Event{
		triggerReconciled:         {Reason: triggerReconciled, Type: corev1.EventTypeNormal},
		triggerUpdateStatusFailed: {Reason: triggerUpdateStatusFailed, Type: corev1.EventTypeWarning},
		triggerReconcileFailed:    {Reason: triggerReconcileFailed, Type: corev1.EventTypeWarning},
		subscriptionDeleteFailed:  {Reason: subscriptionDeleteFailed, Type: corev1.EventTypeWarning},
		subscriptionCreateFailed:  {Reason: subscriptionCreateFailed, Type: corev1.EventTypeWarning},
	}
*/
)

func init() {
	// Add types to scheme
	_ = v1alpha1.AddToScheme(scheme.Scheme)
	_ = duckv1alpha1.AddToScheme(scheme.Scheme)
}

func TestAllCases(t *testing.T) {
	imcKey := testNS + "/" + imcName
	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		}, {
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
		}, { // TODO: there is a bug in the controller, it will query for ""
			//			Name: "trigger key not found ",
			//			Objects: []runtime.Object{
			//				reconciletesting.NewTrigger(triggerName, testNS),
			//			},
			//			Key:     "foo/incomplete",
			//			WantErr: true,
			//			WantEvents: []string{
			//				Eventf(corev1.EventTypeWarning, "ChannelReferenceFetchFailed", "Failed to validate spec.channel exists: s \"\" not found"),
			//			},
		}, {
			Name: "deleting",
			Key:  imcKey,
			Objects: []runtime.Object{
				reconciletesting.NewInMemoryChannel(imcName, testNS,
					reconciletesting.WithInitInMemoryChannelConditions,
					reconciletesting.WithInMemoryChannelDeleted)},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "InMemoryChannelReconciled", "InMemoryChannel reconciled"),
			},
		}, {
			Name: "deployment does not exist",
			Key:  imcKey,
			Objects: []runtime.Object{
				reconciletesting.NewInMemoryChannel(imcName, testNS,
					reconciletesting.WithInitInMemoryChannelConditions,
					reconciletesting.WithInMemoryChannelDeploymentNotFound("DispatcherDeploymentDoesNotExist", "Dispatcher Deployment does not exist"),
				)},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InMemoryChannelReconcileFailed", "InMemoryChannel reconciliation failed: dispatcher Deployment does not exist"),
			},
		}, {
			Name: "Service does not exist",
			Key:  imcKey,
			Objects: []runtime.Object{
				reconciletesting.NewInMemoryChannel(imcName, testNS,
					reconciletesting.WithInitInMemoryChannelConditions,
					reconciletesting.WithInMemoryChannelDeploymentNotFound("DispatcherDeploymentDoesNotExist", "Dispatcher Deployment does not exist"),
				)},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InMemoryChannelReconcileFailed", "InMemoryChannel reconciliation failed: dispatcher Deployment does not exist"),
			},
		}, {},
	}
	defer logtesting.ClearAll()

	table.Test(t, reconciletesting.MakeFactory(func(listers *reconciletesting.Listers, opt reconciler.Options) controller.Reconciler {
		return &Reconciler{
			Base:                     reconciler.NewBase(opt, controllerAgentName),
			dispatcherNamespace:      testNS,
			dispatcherDeploymentName: dispatcherDeploymentName,
			dispatcherServiceName:    dispatcherServiceName,
			inmemorychannelLister:    listers.GetInMemoryChannelLister(),
			// TODO: FIx
			inmemorychannelInformer: nil,
			deploymentLister:        listers.GetDeploymentLister(),
			serviceLister:           listers.GetServiceLister(),
			endpointsLister:         listers.GetEndpointsLister(),
		}

	}))
}

func makeDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      dispatcherDeploymentName,
		},
		Status: appsv1.DeploymentStatus{},
	}
}

func makeReadyDeployment() *appsv1.Deployment {
	b := makeDeployment()
	return b
}
