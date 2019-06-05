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

package controller

import (
	"fmt"
	"github.com/knative/eventing/pkg/reconciler/inmemorychannel/controller/resources"
	"testing"

	"github.com/knative/eventing/pkg/apis/messaging/v1alpha1"
	"github.com/knative/eventing/pkg/reconciler"
	reconciletesting "github.com/knative/eventing/pkg/reconciler/testing"
	"github.com/knative/eventing/pkg/utils"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/kmeta"
	logtesting "github.com/knative/pkg/logging/testing"
	. "github.com/knative/pkg/reconciler/testing"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
)

const (
	systemNS                 = "knative-eventing"
	testNS                   = "test-namespace"
	imcName                  = "test-imc"
	dispatcherDeploymentName = "test-deployment"
	dispatcherServiceName    = "test-service"
	channelServiceAddress    = "test-imc-kn-channel.test-namespace.svc.cluster.local"

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
				Eventf(corev1.EventTypeNormal, "Reconciled", "InMemoryChannel reconciled"),
			},
		}, {
			Name: "deployment does not exist",
			Key:  imcKey,
			Objects: []runtime.Object{
				reconciletesting.NewInMemoryChannel(imcName, testNS),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewInMemoryChannel(imcName, testNS,
					reconciletesting.WithInitInMemoryChannelConditions,
					reconciletesting.WithInMemoryChannelDeploymentNotReady("DispatcherDeploymentDoesNotExist", "Dispatcher Deployment does not exist")),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "ReconcileFailed", "InMemoryChannel reconciliation failed: deployment.apps \"test-deployment\" not found"),
			},
		}, {
			Name: "Service does not exist",
			Key:  imcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				reconciletesting.NewInMemoryChannel(imcName, testNS),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewInMemoryChannel(imcName, testNS,
					reconciletesting.WithInitInMemoryChannelConditions,
					reconciletesting.WithInMemoryChannelDeploymentReady(),
					reconciletesting.WithInMemoryChannelServicetNotReady("DispatcherServiceDoesNotExist", "Dispatcher Service does not exist")),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "ReconcileFailed", "InMemoryChannel reconciliation failed: service \"test-service\" not found"),
			},
		}, {
			Name: "Endpoints does not exist",
			Key:  imcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				reconciletesting.NewInMemoryChannel(imcName, testNS),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewInMemoryChannel(imcName, testNS,
					reconciletesting.WithInitInMemoryChannelConditions,
					reconciletesting.WithInMemoryChannelDeploymentReady(),
					reconciletesting.WithInMemoryChannelServiceReady(),
					reconciletesting.WithInMemoryChannelEndpointsNotReady("DispatcherEndpointsDoesNotExist", "Dispatcher Endpoints does not exist"),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "ReconcileFailed", "InMemoryChannel reconciliation failed: endpoints \"test-service\" not found"),
			},
		}, {
			Name: "Endpoints not ready",
			Key:  imcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeEmptyEndpoints(),
				reconciletesting.NewInMemoryChannel(imcName, testNS),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewInMemoryChannel(imcName, testNS,
					reconciletesting.WithInitInMemoryChannelConditions,
					reconciletesting.WithInMemoryChannelDeploymentReady(),
					reconciletesting.WithInMemoryChannelServiceReady(),
					reconciletesting.WithInMemoryChannelEndpointsNotReady("DispatcherEndpointsNotReady", "There are no endpoints ready for Dispatcher service"),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "ReconcileFailed", "InMemoryChannel reconciliation failed: there are no endpoints ready for Dispatcher service"),
			},
		}, {
			Name: "Works, creates new channel",
			Key:  imcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				reconciletesting.NewInMemoryChannel(imcName, testNS),
			},
			WantErr: false,
			WantCreates: []runtime.Object{
				makeChannelService(reconciletesting.NewInMemoryChannel(imcName, testNS)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewInMemoryChannel(imcName, testNS,
					reconciletesting.WithInitInMemoryChannelConditions,
					reconciletesting.WithInMemoryChannelDeploymentReady(),
					reconciletesting.WithInMemoryChannelServiceReady(),
					reconciletesting.WithInMemoryChannelEndpointsReady(),
					reconciletesting.WithInMemoryChannelChannelServiceReady(),
					reconciletesting.WithInMemoryChannelAddress(channelServiceAddress),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "InMemoryChannel reconciled"),
			},
		}, {
			Name: "Works, channel exists",
			Key:  imcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				reconciletesting.NewInMemoryChannel(imcName, testNS),
				makeChannelService(reconciletesting.NewInMemoryChannel(imcName, testNS)),
			},
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewInMemoryChannel(imcName, testNS,
					reconciletesting.WithInitInMemoryChannelConditions,
					reconciletesting.WithInMemoryChannelDeploymentReady(),
					reconciletesting.WithInMemoryChannelServiceReady(),
					reconciletesting.WithInMemoryChannelEndpointsReady(),
					reconciletesting.WithInMemoryChannelChannelServiceReady(),
					reconciletesting.WithInMemoryChannelAddress(channelServiceAddress),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "InMemoryChannel reconciled"),
			},
		}, {
			Name: "channel exists, not owned by us",
			Key:  imcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				reconciletesting.NewInMemoryChannel(imcName, testNS),
				makeChannelServiceNotOwnedByUs(reconciletesting.NewInMemoryChannel(imcName, testNS)),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewInMemoryChannel(imcName, testNS,
					reconciletesting.WithInitInMemoryChannelConditions,
					reconciletesting.WithInMemoryChannelDeploymentReady(),
					reconciletesting.WithInMemoryChannelServiceReady(),
					reconciletesting.WithInMemoryChannelEndpointsReady(),
					reconciletesting.WithInMemoryChannelChannelServicetNotReady("ChannelServiceFailed", "Channel Service failed: inmemorychannel: test-namespace/test-imc does not own Service: \"test-imc-kn-channel\""),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "ReconcileFailed", "InMemoryChannel reconciliation failed: inmemorychannel: test-namespace/test-imc does not own Service: \"test-imc-kn-channel\""),
			},
		}, {
			Name: "channel does not exist, fails to create",
			Key:  imcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				reconciletesting.NewInMemoryChannel(imcName, testNS),
			},
			WantErr: true,
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "Services"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewInMemoryChannel(imcName, testNS,
					reconciletesting.WithInitInMemoryChannelConditions,
					reconciletesting.WithInMemoryChannelDeploymentReady(),
					reconciletesting.WithInMemoryChannelServiceReady(),
					reconciletesting.WithInMemoryChannelEndpointsReady(),
					reconciletesting.WithInMemoryChannelChannelServicetNotReady("ChannelServiceFailed", "Channel Service failed: inducing failure for create services"),
				),
			}},
			WantCreates: []runtime.Object{
				makeChannelService(reconciletesting.NewInMemoryChannel(imcName, testNS)),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "ReconcileFailed", "InMemoryChannel reconciliation failed: inducing failure for create services"),
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
	},
		false,
	))
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
	d := makeDeployment()
	d.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue}}
	return d
}

func makeService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      dispatcherServiceName,
		},
	}
}

func makeChannelService(imc *v1alpha1.InMemoryChannel) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      fmt.Sprintf("%s-kn-channel", imcName),
			Labels: map[string]string{
				resources.MessagingRoleLabel: resources.MessagingRole,
			},
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(imc),
			},
		},
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: fmt.Sprintf("%s.%s.svc.%s", dispatcherServiceName, testNS, utils.GetClusterDomainName()),
		},
	}
}

func makeChannelServiceNotOwnedByUs(imc *v1alpha1.InMemoryChannel) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      fmt.Sprintf("%s-kn-channel", imcName),
			Labels: map[string]string{
				resources.MessagingRoleLabel: resources.MessagingRole,
			},
		},
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: fmt.Sprintf("%s.%s.svc.%s", dispatcherServiceName, testNS, utils.GetClusterDomainName()),
		},
	}
}

func makeEmptyEndpoints() *corev1.Endpoints {
	return &corev1.Endpoints{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Endpoints",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      dispatcherServiceName,
		},
	}
}

func makeReadyEndpoints() *corev1.Endpoints {
	e := makeEmptyEndpoints()
	e.Subsets = []corev1.EndpointSubset{{Addresses: []corev1.EndpointAddress{{IP: "1.1.1.1"}}}}
	return e
}
