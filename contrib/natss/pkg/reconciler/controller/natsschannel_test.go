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
	"testing"

	"github.com/knative/eventing/pkg/utils"
	"knative.dev/pkg/kmeta"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/knative/eventing/contrib/natss/pkg/apis/messaging/v1alpha1"
	fakeclientset "github.com/knative/eventing/contrib/natss/pkg/client/clientset/versioned/fake"
	informers "github.com/knative/eventing/contrib/natss/pkg/client/informers/externalversions"
	"github.com/knative/eventing/contrib/natss/pkg/reconciler"
	"github.com/knative/eventing/contrib/natss/pkg/reconciler/controller/resources"
	reconciletesting "github.com/knative/eventing/contrib/natss/pkg/reconciler/testing"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
)

const (
	systemNS                 = "knative-eventing"
	testNS                   = "test-namespace"
	ncName                   = "test-nc"
	dispatcherDeploymentName = "test-deployment"
	dispatcherServiceName    = "test-service"
	channelServiceAddress    = "test-nc-kn-channel.test-namespace.svc.cluster.local"
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

func TestNewController(t *testing.T) {
	kubeClient := fakekubeclientset.NewSimpleClientset()
	messagingClient := fakeclientset.NewSimpleClientset()

	// Create informer factories with fake clients. The second parameter sets the
	// resync period to zero, disabling it.
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
	messagingInformerFactory := informers.NewSharedInformerFactory(messagingClient, 0)

	// Eventing
	ncInformer := messagingInformerFactory.Messaging().V1alpha1().NatssChannels()

	// Kube
	serviceInformer := kubeInformerFactory.Core().V1().Services()
	endpointsInformer := kubeInformerFactory.Core().V1().Endpoints()
	deploymentInformer := kubeInformerFactory.Apps().V1().Deployments()

	c := NewController(
		reconciler.Options{
			KubeClientSet:  kubeClient,
			NatssClientSet: messagingClient,
			Logger:         logtesting.TestLogger(t),
		},
		systemNS,
		dispatcherDeploymentName,
		dispatcherServiceName,
		ncInformer,
		deploymentInformer,
		serviceInformer,
		endpointsInformer)

	if c == nil {
		t.Fatalf("Failed to create with NewController")
	}
}

func TestAllCases(t *testing.T) {
	ncKey := testNS + "/" + ncName
	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		}, {
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
		}, {
			Name: "deleting",
			Key:  ncKey,
			Objects: []runtime.Object{
				reconciletesting.NewNatssChannel(ncName, testNS,
					reconciletesting.WithNatssInitChannelConditions,
					reconciletesting.WithNatssChannelDeleted)},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, channelReconciled, "NatssChannel reconciled"),
			},
		}, {
			Name: "deployment does not exist",
			Key:  ncKey,
			Objects: []runtime.Object{
				reconciletesting.NewNatssChannel(ncName, testNS),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewNatssChannel(ncName, testNS,
					reconciletesting.WithNatssInitChannelConditions,
					reconciletesting.WithNatssChannelDeploymentNotReady("DispatcherDeploymentDoesNotExist", "Dispatcher Deployment does not exist")),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, channelReconcileFailed, "NatssChannel reconciliation failed: deployment.apps \"test-deployment\" not found"),
			},
		}, {
			Name: "Service does not exist",
			Key:  ncKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				reconciletesting.NewNatssChannel(ncName, testNS),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewNatssChannel(ncName, testNS,
					reconciletesting.WithNatssInitChannelConditions,
					reconciletesting.WithNatssChannelDeploymentReady(),
					reconciletesting.WithNatssChannelServicetNotReady("DispatcherServiceDoesNotExist", "Dispatcher Service does not exist")),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, channelReconcileFailed, "NatssChannel reconciliation failed: service \"test-service\" not found"),
			},
		}, {
			Name: "Endpoints does not exist",
			Key:  ncKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				reconciletesting.NewNatssChannel(ncName, testNS),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewNatssChannel(ncName, testNS,
					reconciletesting.WithNatssInitChannelConditions,
					reconciletesting.WithNatssChannelDeploymentReady(),
					reconciletesting.WithNatssChannelServiceReady(),
					reconciletesting.WithNatssChannelEndpointsNotReady("DispatcherEndpointsDoesNotExist", "Dispatcher Endpoints does not exist"),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, channelReconcileFailed, "NatssChannel reconciliation failed: endpoints \"test-service\" not found"),
			},
		}, {
			Name: "Endpoints not ready",
			Key:  ncKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeEmptyEndpoints(),
				reconciletesting.NewNatssChannel(ncName, testNS),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewNatssChannel(ncName, testNS,
					reconciletesting.WithNatssInitChannelConditions,
					reconciletesting.WithNatssChannelDeploymentReady(),
					reconciletesting.WithNatssChannelServiceReady(),
					reconciletesting.WithNatssChannelEndpointsNotReady("DispatcherEndpointsNotReady", "There are no endpoints ready for Dispatcher service"),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, channelReconcileFailed, "NatssChannel reconciliation failed: there are no endpoints ready for Dispatcher service test-service"),
			},
		}, {
			Name: "Works, creates new channel",
			Key:  ncKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				reconciletesting.NewNatssChannel(ncName, testNS),
			},
			WantErr: false,
			WantCreates: []runtime.Object{
				makeChannelService(reconciletesting.NewNatssChannel(ncName, testNS)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewNatssChannel(ncName, testNS,
					reconciletesting.WithNatssInitChannelConditions,
					reconciletesting.WithNatssChannelDeploymentReady(),
					reconciletesting.WithNatssChannelServiceReady(),
					reconciletesting.WithNatssChannelEndpointsReady(),
					reconciletesting.WithNatssChannelChannelServiceReady(),
					reconciletesting.WithNatssChannelAddress(channelServiceAddress),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, channelReconciled, "NatssChannel reconciled"),
			},
		}, {
			Name: "Works, channel exists",
			Key:  ncKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				reconciletesting.NewNatssChannel(ncName, testNS),
				makeChannelService(reconciletesting.NewNatssChannel(ncName, testNS)),
			},
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewNatssChannel(ncName, testNS,
					reconciletesting.WithNatssInitChannelConditions,
					reconciletesting.WithNatssChannelDeploymentReady(),
					reconciletesting.WithNatssChannelServiceReady(),
					reconciletesting.WithNatssChannelEndpointsReady(),
					reconciletesting.WithNatssChannelChannelServiceReady(),
					reconciletesting.WithNatssChannelAddress(channelServiceAddress),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, channelReconciled, "NatssChannel reconciled"),
			},
		}, {
			Name: "channel exists, not owned by us",
			Key:  ncKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				reconciletesting.NewNatssChannel(ncName, testNS),
				makeChannelServiceNotOwnedByUs(reconciletesting.NewNatssChannel(ncName, testNS)),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewNatssChannel(ncName, testNS,
					reconciletesting.WithNatssInitChannelConditions,
					reconciletesting.WithNatssChannelDeploymentReady(),
					reconciletesting.WithNatssChannelServiceReady(),
					reconciletesting.WithNatssChannelEndpointsReady(),
					reconciletesting.WithNatssChannelChannelServicetNotReady("ChannelServiceFailed", "Channel Service failed: natsschannel: test-namespace/test-nc does not own Service: \"test-nc-kn-channel\""),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, channelReconcileFailed, "NatssChannel reconciliation failed: natsschannel: test-namespace/test-nc does not own Service: \"test-nc-kn-channel\""),
			},
		}, {
			Name: "channel does not exist, fails to create",
			Key:  ncKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				reconciletesting.NewNatssChannel(ncName, testNS),
			},
			WantErr: true,
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "Services"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewNatssChannel(ncName, testNS,
					reconciletesting.WithNatssInitChannelConditions,
					reconciletesting.WithNatssChannelDeploymentReady(),
					reconciletesting.WithNatssChannelServiceReady(),
					reconciletesting.WithNatssChannelEndpointsReady(),
					reconciletesting.WithNatssChannelChannelServicetNotReady("ChannelServiceFailed", "Channel Service failed: inducing failure for create services"),
				),
			}},
			WantCreates: []runtime.Object{
				makeChannelService(reconciletesting.NewNatssChannel(ncName, testNS)),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, channelReconcileFailed, "NatssChannel reconciliation failed: inducing failure for create services"),
			},
		},
	}
	defer logtesting.ClearAll()

	table.Test(t, reconciletesting.MakeFactory(func(listers *reconciletesting.Listers, opt reconciler.Options) controller.Reconciler {
		return &Reconciler{
			Base:                     reconciler.NewBase(opt, controllerAgentName),
			dispatcherNamespace:      testNS,
			dispatcherDeploymentName: dispatcherDeploymentName,
			dispatcherServiceName:    dispatcherServiceName,
			natsschannelLister:       listers.GetNatssChannelLister(),
			// TODO fix
			natsschannelInformer: nil,
			deploymentLister:     listers.GetDeploymentLister(),
			serviceLister:        listers.GetServiceLister(),
			endpointsLister:      listers.GetEndpointsLister(),
		}
	},
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

func makeChannelService(nc *v1alpha1.NatssChannel) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      fmt.Sprintf("%s-kn-channel", ncName),
			Labels: map[string]string{
				resources.MessagingRoleLabel: resources.MessagingRole,
			},
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(nc),
			},
		},
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: fmt.Sprintf("%s.%s.svc.%s", dispatcherServiceName, testNS, utils.GetClusterDomainName()),
		},
	}
}

func makeChannelServiceNotOwnedByUs(nc *v1alpha1.NatssChannel) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      fmt.Sprintf("%s-kn-channel", ncName),
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
