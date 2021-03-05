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
	"context"
	"fmt"
	"strconv"
	"testing"

	"k8s.io/apimachinery/pkg/types"

	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/reconciler/inmemorychannel/controller/config"

	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/network"

	"knative.dev/eventing/pkg/apis/eventing"

	"knative.dev/eventing/pkg/client/injection/reconciler/messaging/v1/inmemorychannel"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	v1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/eventing/pkg/reconciler/inmemorychannel/controller/resources"
	. "knative.dev/eventing/pkg/reconciler/testing/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	systemNS              = "knative-testing"
	testNS                = "test-namespace"
	imcName               = "test-imc"
	channelServiceAddress = "test-imc-kn-channel.test-namespace.svc.cluster.local"
	imageName             = "test-image"
	maxIdleConns          = 2000
	maxIdleConnsPerHost   = 200

	imcGeneration = 7
)

func init() {
	// Add types to scheme
	_ = v1.AddToScheme(scheme.Scheme)
	_ = duckv1.AddToScheme(scheme.Scheme)
}

func TestAllCases(t *testing.T) {
	imcKey := testNS + "/" + imcName
	subscriber1UID := types.UID("2f9b5e8e-deb6-11e8-9f32-f2801f1b9fd1")
	subscriber2UID := types.UID("34c5aec8-deb6-11e8-9f32-f2801f1b9fd1")
	subscriber1Generation := int64(1)
	subscriber2Generation := int64(2)

	subscribers := []eventingduckv1.SubscriberSpec{{
		UID:           subscriber1UID,
		Generation:    subscriber1Generation,
		SubscriberURI: apis.HTTP("call1"),
		ReplyURI:      apis.HTTP("sink2"),
	}, {
		UID:           subscriber2UID,
		Generation:    subscriber2Generation,
		SubscriberURI: apis.HTTP("call2"),
		ReplyURI:      apis.HTTP("sink2"),
	}}

	subscriberStatuses := []eventingduckv1.SubscriberStatus{{
		UID:                subscriber1UID,
		ObservedGeneration: subscriber1Generation,
		Ready:              "True",
	}, {
		UID:                subscriber2UID,
		ObservedGeneration: subscriber2Generation,
		Ready:              "True",
	}}

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
			Key:  imcKey,
			Objects: []runtime.Object{
				NewInMemoryChannel(imcName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelDeleted)},
			WantErr: false,
		}, {
			Name: "deployment does not exist",
			Key:  imcKey,
			Objects: []runtime.Object{
				NewInMemoryChannel(imcName, testNS),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewInMemoryChannel(imcName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelDeploymentFailed("DispatcherDeploymentDoesNotExist", "Dispatcher Deployment does not exist")),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "DispatcherDeploymentFailed", `Reconciling dispatcher Deployment failed with: deployment.apps "imc-dispatcher" not found`),
			},
		}, {
			Name: "the status of deployment is false",
			Key:  imcKey,
			Objects: []runtime.Object{
				makeFalseDeployment(),
				makeService(),
				makeReadyEndpoints(),
				NewInMemoryChannel(imcName, testNS),
			},
			WantErr: false,
			WantCreates: []runtime.Object{
				makeChannelService(NewInMemoryChannel(imcName, testNS)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewInMemoryChannel(imcName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelDeploymentFailed("DispatcherDeploymentFalse", "The status of Dispatcher Deployment is False: Deployment Failed : Deployment Failed"),
					WithInMemoryChannelServiceReady(),
					WithInMemoryChannelEndpointsReady(),
					WithInMemoryChannelChannelServiceReady(),
					WithInMemoryChannelAddress(channelServiceAddress)),
			}},
		}, {
			Name: "the status of deployment is unknown",
			Key:  imcKey,
			Objects: []runtime.Object{
				makeUnknownDeployment(),
				makeService(),
				makeReadyEndpoints(),
				NewInMemoryChannel(imcName, testNS),
			},
			WantErr: false,
			WantCreates: []runtime.Object{
				makeChannelService(NewInMemoryChannel(imcName, testNS)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewInMemoryChannel(imcName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelDeploymentUnknown("DispatcherDeploymentUnknown", "The status of Dispatcher Deployment is Unknown: Deployment Unknown : Deployment Unknown"),
					WithInMemoryChannelServiceReady(),
					WithInMemoryChannelEndpointsReady(),
					WithInMemoryChannelChannelServiceReady(),
					WithInMemoryChannelAddress(channelServiceAddress)),
			}},
		}, {
			Name: "Service does not exist",
			Key:  imcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				NewInMemoryChannel(imcName, testNS),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewInMemoryChannel(imcName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelDeploymentReady(),
					WithInMemoryChannelServicetNotReady("DispatcherServiceDoesNotExist", "Dispatcher Service does not exist")),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "DispatcherServiceFailed", `Reconciling dispatcher Service failed: service "imc-dispatcher" not found`),
			},
		}, {
			Name: "Endpoints does not exist",
			Key:  imcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				NewInMemoryChannel(imcName, testNS),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewInMemoryChannel(imcName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelDeploymentReady(),
					WithInMemoryChannelServiceReady(),
					WithInMemoryChannelEndpointsNotReady("DispatcherEndpointsDoesNotExist", "Dispatcher Endpoints does not exist"),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", `endpoints "imc-dispatcher" not found`),
			},
		}, {
			Name: "Endpoints not ready",
			Key:  imcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeEmptyEndpoints(),
				NewInMemoryChannel(imcName, testNS),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewInMemoryChannel(imcName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelDeploymentReady(),
					WithInMemoryChannelServiceReady(),
					WithInMemoryChannelEndpointsNotReady("DispatcherEndpointsNotReady", "There are no endpoints ready for Dispatcher service"),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", `there are no endpoints ready for Dispatcher service`),
			},
		}, {
			Name: "Works, creates new channel",
			Key:  imcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				NewInMemoryChannel(imcName, testNS,
					WithInMemoryChannelGeneration(imcGeneration)),
			},
			WantErr: false,
			WantCreates: []runtime.Object{
				makeChannelService(NewInMemoryChannel(imcName, testNS)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewInMemoryChannel(imcName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelDeploymentReady(),
					WithInMemoryChannelGeneration(imcGeneration),
					WithInMemoryChannelStatusObservedGeneration(imcGeneration),
					WithInMemoryChannelServiceReady(),
					WithInMemoryChannelEndpointsReady(),
					WithInMemoryChannelChannelServiceReady(),
					WithInMemoryChannelAddress(channelServiceAddress),
				),
			}},
		}, {
			Name: "Works, channel exists",
			Key:  imcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				NewInMemoryChannel(imcName, testNS),
				makeChannelService(NewInMemoryChannel(imcName, testNS)),
			},
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewInMemoryChannel(imcName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelDeploymentReady(),
					WithInMemoryChannelServiceReady(),
					WithInMemoryChannelEndpointsReady(),
					WithInMemoryChannelChannelServiceReady(),
					WithInMemoryChannelAddress(channelServiceAddress),
				),
			}},
		}, {
			Name: "channel exists, not owned by us",
			Key:  imcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				NewInMemoryChannel(imcName, testNS),
				makeChannelServiceNotOwnedByUs(NewInMemoryChannel(imcName, testNS)),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewInMemoryChannel(imcName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelDeploymentReady(),
					WithInMemoryChannelServiceReady(),
					WithInMemoryChannelEndpointsReady(),
					WithInMemoryChannelChannelServiceNotReady("ChannelServiceFailed", `Channel Service failed: inmemorychannel: test-namespace/test-imc does not own Service: "test-imc-kn-channel"`),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", `inmemorychannel: test-namespace/test-imc does not own Service: "test-imc-kn-channel"`),
			},
		}, {
			Name: "Works, channel exists with subscribers",
			Key:  imcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				NewInMemoryChannel(imcName, testNS,
					WithInMemoryChannelSubscribers(subscribers)),
				makeChannelService(NewInMemoryChannel(imcName, testNS)),
			},
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewInMemoryChannel(imcName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelDeploymentReady(),
					WithInMemoryChannelServiceReady(),
					WithInMemoryChannelEndpointsReady(),
					WithInMemoryChannelChannelServiceReady(),
					WithInMemoryChannelSubscribers(subscribers),
					WithInMemoryChannelAddress(channelServiceAddress),
				),
			}},
		}, {
			Name: "Works, channel exists with subscribers, in status, not modified",
			Key:  imcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				NewInMemoryChannel(imcName, testNS,
					WithInMemoryChannelSubscribers(subscribers),
					WithInMemoryChannelReadySubscriberAndGeneration(string(subscriber1UID), subscriber1Generation),
					WithInMemoryChannelReadySubscriberAndGeneration(string(subscriber2UID), subscriber2Generation)),
				makeChannelService(NewInMemoryChannel(imcName, testNS)),
			},
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewInMemoryChannel(imcName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelDeploymentReady(),
					WithInMemoryChannelServiceReady(),
					WithInMemoryChannelEndpointsReady(),
					WithInMemoryChannelChannelServiceReady(),
					WithInMemoryChannelSubscribers(subscribers),
					WithInMemoryChannelStatusSubscribers(subscriberStatuses),
					WithInMemoryChannelAddress(channelServiceAddress),
				),
			}},
		}, {
			Name: "channel does not exist, fails to create",
			Key:  imcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				NewInMemoryChannel(imcName, testNS),
			},
			WantErr: true,
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "Services"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewInMemoryChannel(imcName, testNS,
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelDeploymentReady(),
					WithInMemoryChannelServiceReady(),
					WithInMemoryChannelEndpointsReady(),
					WithInMemoryChannelChannelServiceNotReady("ChannelServiceFailed", "Channel Service failed: inducing failure for create services"),
				),
			}},
			WantCreates: []runtime.Object{
				makeChannelService(NewInMemoryChannel(imcName, testNS)),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for create services"),
			},
		}, {},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		r := &Reconciler{
			kubeClientSet:    fakekubeclient.Get(ctx),
			systemNamespace:  testNS,
			deploymentLister: listers.GetDeploymentLister(),
			serviceLister:    listers.GetServiceLister(),
			endpointsLister:  listers.GetEndpointsLister(),
		}
		return inmemorychannel.NewReconciler(ctx, logger,
			fakeeventingclient.Get(ctx), listers.GetInMemoryChannelLister(),
			controller.GetEventRecorder(ctx), r)
	},
		false,
		logger,
	))
}

func TestInNamespace(t *testing.T) {
	imcKey := testNS + "/" + imcName
	table := TableTest{
		{
			Name: "Works, creates new service account, role binding, dispatcher deployment and service and channel",
			Key:  imcKey,
			Objects: []runtime.Object{
				makeEventDispatcherConfigMap(),
				NewInMemoryChannel(imcName, testNS, WithInMemoryScopeAnnotation(eventing.ScopeNamespace)),
				makeRoleBinding(systemNS, dispatcherName+"-"+testNS, "eventing-config-reader", NewInMemoryChannel(imcName, testNS)),
				makeReadyEndpoints(),
			},
			WantErr: false,
			WantCreates: []runtime.Object{
				makeServiceAccount(NewInMemoryChannel(imcName, testNS)),
				makeRoleBinding(testNS, dispatcherName, dispatcherName, NewInMemoryChannel(imcName, testNS)),
				makeDispatcherDeployment(NewInMemoryChannel(imcName, testNS)),
				makeDispatcherService(testNS),
				makeChannelService(NewInMemoryChannel(imcName, testNS)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewInMemoryChannel(imcName, testNS,
					WithInMemoryScopeAnnotation(eventing.ScopeNamespace),
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelServiceReady(),
					WithInMemoryChannelEndpointsReady(),
					WithInMemoryChannelChannelServiceReady(),
					WithInMemoryChannelAddress(channelServiceAddress),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "DispatcherServiceAccountCreated", "Dispatcher ServiceAccount created"),
				Eventf(corev1.EventTypeNormal, "DispatcherRoleBindingCreated", "Dispatcher RoleBinding created"),
				Eventf(corev1.EventTypeNormal, "DispatcherDeploymentCreated", "Dispatcher Deployment created"),
				Eventf(corev1.EventTypeNormal, "DispatcherServiceCreated", "Dispatcher Service created"),
			},
		},
		{
			Name: "Works, existing service account, role binding, dispatcher deployment and service, new channel",
			Key:  imcKey,
			Objects: []runtime.Object{
				makeEventDispatcherConfigMap(),
				NewInMemoryChannel(imcName, testNS, WithInMemoryScopeAnnotation(eventing.ScopeNamespace)),
				makeServiceAccount(NewInMemoryChannel(imcName, testNS)),
				makeRoleBinding(testNS, dispatcherName, dispatcherName, NewInMemoryChannel(imcName, testNS)),
				makeRoleBinding(systemNS, dispatcherName+"-"+testNS, "eventing-config-reader", NewInMemoryChannel(imcName, "knative-testing")),
				makeDispatcherDeployment(NewInMemoryChannel(imcName, testNS)),
				makeDispatcherService(testNS),
				makeReadyEndpoints(),
			},
			WantErr: false,
			WantCreates: []runtime.Object{
				makeChannelService(NewInMemoryChannel(imcName, testNS)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewInMemoryChannel(imcName, testNS,
					WithInMemoryScopeAnnotation(eventing.ScopeNamespace),
					WithInitInMemoryChannelConditions,
					WithInMemoryChannelServiceReady(),
					WithInMemoryChannelEndpointsReady(),
					WithInMemoryChannelChannelServiceReady(),
					WithInMemoryChannelAddress(channelServiceAddress),
				),
			}},
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		eventDispatcherConfigStore := config.NewEventDispatcherConfigStore(logger)
		eventDispatcherConfigStore.WatchConfigs(cmw)

		r := &Reconciler{
			kubeClientSet:              fakekubeclient.Get(ctx),
			dispatcherImage:            imageName,
			systemNamespace:            systemNS,
			deploymentLister:           listers.GetDeploymentLister(),
			serviceLister:              listers.GetServiceLister(),
			endpointsLister:            listers.GetEndpointsLister(),
			serviceAccountLister:       listers.GetServiceAccountLister(),
			roleBindingLister:          listers.GetRoleBindingLister(),
			eventDispatcherConfigStore: eventDispatcherConfigStore,
		}
		return inmemorychannel.NewReconciler(ctx, logger,
			fakeeventingclient.Get(ctx), listers.GetInMemoryChannelLister(),
			controller.GetEventRecorder(ctx), r)
	},
		false,
		logger,
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
			Name:      dispatcherName,
		},
		Status: appsv1.DeploymentStatus{},
	}
}

func makeReadyDeployment() *appsv1.Deployment {
	d := makeDeployment()
	d.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue}}
	return d
}

func makeFalseDeployment() *appsv1.Deployment {
	d := makeDeployment()
	d.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionFalse, Reason: "Deployment Failed", Message: "Deployment Failed"}}
	return d
}

func makeUnknownDeployment() *appsv1.Deployment {
	d := makeDeployment()
	d.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionUnknown, Reason: "Deployment Unknown", Message: "Deployment Unknown"}}
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
			Name:      dispatcherName,
		},
	}
}

func makeServiceAccount(imc *v1.InMemoryChannel) *corev1.ServiceAccount {
	return resources.MakeServiceAccount(imc.Namespace, dispatcherName)
}

func makeRoleBinding(ns, name, clusterRoleName string, imc *v1.InMemoryChannel) *rbacv1.RoleBinding {
	return resources.MakeRoleBinding(ns, name, makeServiceAccount(imc), clusterRoleName)
}

func makeDispatcherDeployment(imc *v1.InMemoryChannel) *appsv1.Deployment {
	return resources.MakeDispatcher(resources.DispatcherArgs{
		EventDispatcherConfig: config.EventDispatcherConfig{
			ConnectionArgs: kncloudevents.ConnectionArgs{
				MaxIdleConns:        maxIdleConns,
				MaxIdleConnsPerHost: maxIdleConnsPerHost,
			},
		},
		DispatcherName:      dispatcherName,
		DispatcherNamespace: testNS,
		Image:               imageName,
		ServiceAccountName:  dispatcherName,
	})
}

func makeDispatcherService(ns string) *corev1.Service {
	return resources.MakeDispatcherService(dispatcherName, ns)
}

func makeChannelService(imc *v1.InMemoryChannel) *corev1.Service {
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
			ExternalName: network.GetServiceHostname(dispatcherName, testNS),
		},
	}
}

func makeChannelServiceNotOwnedByUs(imc *v1.InMemoryChannel) *corev1.Service {
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
			ExternalName: network.GetServiceHostname(dispatcherName, testNS),
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
			Name:      dispatcherName,
		},
	}
}

func makeReadyEndpoints() *corev1.Endpoints {
	e := makeEmptyEndpoints()
	e.Subsets = []corev1.EndpointSubset{{Addresses: []corev1.EndpointAddress{{IP: "1.1.1.1"}}}}
	return e
}

func makeEventDispatcherConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.EventDispatcherConfigMap,
			Namespace: systemNS,
		},
		Data: map[string]string{
			"MaxIdleConnections":        strconv.Itoa(maxIdleConns),
			"MaxIdleConnectionsPerHost": strconv.Itoa(maxIdleConnsPerHost),
		},
	}
}
