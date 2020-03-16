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

package broker

import (
	"context"
	"fmt"
	"testing"

	sourcesv1alpha2 "knative.dev/eventing/pkg/apis/sources/v1alpha2"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"

	clientgotesting "k8s.io/client-go/testing"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/eventing/pkg/client/injection/ducks/duck/v1alpha1/channelable"
	"knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1alpha1/broker"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/eventing/pkg/reconciler/broker/resources"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	v1addr "knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/client/injection/ducks/duck/v1/conditions"
	v1a1addr "knative.dev/pkg/client/injection/ducks/duck/v1alpha1/addressable"
	v1b1addr "knative.dev/pkg/client/injection/ducks/duck/v1beta1/addressable"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/system"

	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/trigger/fake"
	. "knative.dev/eventing/pkg/reconciler/testing"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"
	. "knative.dev/pkg/reconciler/testing"
)

type channelType string

const (
	testNS     = "test-namespace"
	brokerName = "test-broker"

	filterImage  = "filter-image"
	filterSA     = "filter-SA"
	ingressImage = "ingress-image"
	ingressSA    = "ingress-SA"

	filterContainerName  = "filter"
	ingressContainerName = "ingress"

	triggerChannel channelType = "TriggerChannel"
	triggerName                = "test-trigger"
	triggerUID                 = "test-trigger-uid"

	subscriberURI     = "http://example.com/subscriber/"
	subscriberKind    = "Service"
	subscriberName    = "subscriber-name"
	subscriberGroup   = "serving.knative.dev"
	subscriberVersion = "v1"

	brokerGeneration = 79

	pingSourceName              = "test-ping-source"
	testSchedule                = "*/2 * * * *"
	testData                    = "data"
	sinkName                    = "testsink"
	dependencyAnnotation        = "{\"kind\":\"PingSource\",\"name\":\"test-ping-source\",\"apiVersion\":\"sources.knative.dev/v1alpha2\"}"
	subscriberURIReference      = "foo"
	subscriberResolvedTargetURI = "http://example.com/subscriber/foo"

	k8sServiceResolvedURI = "http://subscriber-name.test-namespace.svc.cluster.local/"
	currentGeneration     = 1
	outdatedGeneration    = 0
	triggerGeneration     = 7

	finalizerName = "brokers.eventing.knative.dev"
)

var (
	trueVal = true

	testKey                 = fmt.Sprintf("%s/%s", testNS, brokerName)
	channelGenerateName     = fmt.Sprintf("%s-broker-", brokerName)
	subscriptionChannelName = fmt.Sprintf("%s-broker", brokerName)

	triggerChannelHostname = fmt.Sprintf("foo.bar.svc.%s", utils.GetClusterDomainName())

	filterDeploymentName  = fmt.Sprintf("%s-broker-filter", brokerName)
	filterServiceName     = fmt.Sprintf("%s-broker-filter", brokerName)
	ingressDeploymentName = fmt.Sprintf("%s-broker-ingress", brokerName)
	ingressServiceName    = fmt.Sprintf("%s-broker", brokerName)

	ingressSubscriptionGenerateName = fmt.Sprintf("internal-ingress-%s-", brokerName)
	subscriptionName                = fmt.Sprintf("%s-%s-%s", brokerName, triggerName, triggerUID)

	channelGVK = metav1.GroupVersionKind{
		Group:   "eventing.knative.dev",
		Version: "v1alpha1",
		Kind:    "Channel",
	}

	imcGVK = metav1.GroupVersionKind{
		Group:   "messaging.knative.dev",
		Version: "v1alpha1",
		Kind:    "InMemoryChannel",
	}

	serviceGVK = metav1.GroupVersionKind{
		Version: "v1",
		Kind:    "Service",
	}
	subscriberAPIVersion = fmt.Sprintf("%s/%s", subscriberGroup, subscriberVersion)
	subscriberGVK        = metav1.GroupVersionKind{
		Group:   subscriberGroup,
		Version: subscriberVersion,
		Kind:    subscriberKind,
	}
	k8sServiceGVK = metav1.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Service",
	}
	brokerDest = duckv1beta1.Destination{
		Ref: &corev1.ObjectReference{
			Name:       sinkName,
			Kind:       "Broker",
			APIVersion: "eventing.knative.dev/v1alpha1",
		},
	}
	brokerDestv1 = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Kind:       "Broker",
			APIVersion: "eventing.knative.dev/v1alpha1",
		},
	}
	sinkDNS               = "sink.mynamespace.svc." + utils.GetClusterDomainName()
	sinkURI               = "http://" + sinkDNS
	finalizerUpdatedEvent = Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-broker" finalizers`)
)

func init() {
	// Add types to scheme
	_ = v1alpha1.AddToScheme(scheme.Scheme)
	_ = duckv1alpha1.AddToScheme(scheme.Scheme)
}

func TestReconcile(t *testing.T) {
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
			Name: "Broker not found",
			Key:  testKey,
		}, {
			Name: "Broker is being deleted",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions,
					WithBrokerDeletionTimestamp),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "BrokerReconciled", `Broker reconciled: "test-namespace/test-broker"`),
			},
		}, {
			Name: "nil channeltemplatespec",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithInitBrokerConditions),
			},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeWarning, "InternalError", "Broker.Spec.ChannelTemplate is nil"),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithInitBrokerConditions,
					WithTriggerChannelFailed("ChannelTemplateFailed", "Error on setting up the ChannelTemplate: Broker.Spec.ChannelTemplate is nil")),
			}},
			// This returns an internal error, so it emits an Error
			WantErr: true,
		}, {
			Name: "Trigger Channel.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions),
			},
			WantCreates: []runtime.Object{
				createChannel(testNS, triggerChannel, false),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithInitBrokerConditions,
					WithBrokerChannel(channel()),
					WithTriggerChannelFailed("ChannelFailure", "inducing failure for create inmemorychannels")),
			}},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "inmemorychannels"),
			},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeWarning, "InternalError", "Failed to reconcile trigger channel: %v", "inducing failure for create inmemorychannels"),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantErr: true,
		}, {
			Name: "Trigger Channel.Create no address",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions),
			},
			WantCreates: []runtime.Object{
				createChannel(testNS, triggerChannel, false),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithInitBrokerConditions,
					WithBrokerChannel(channel()),
					WithTriggerChannelFailed("NoAddress", "Channel does not have an address.")),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantEvents: []string{
				finalizerUpdatedEvent,
			},
		}, {
			Name: "Trigger Channel is not yet Addressable",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions),
				createChannel(testNS, triggerChannel, false),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions,
					WithTriggerChannelFailed("NoAddress", "Channel does not have an address.")),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantEvents: []string{
				finalizerUpdatedEvent,
			},
		}, {
			Name: "Filter Deployment.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions),
				createChannel(testNS, triggerChannel, true),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "deployments"),
			},
			WantCreates: []runtime.Object{
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, livenessProbe(), readinessProbe(), envVars(filterContainerName), containerPorts(8080))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions,
					WithTriggerChannelReady(),
					WithBrokerTriggerChannel(createTriggerChannelRef()),
					WithFilterFailed("DeploymentFailure", "inducing failure for create deployments")),
			}},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for create deployments"),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantErr: true,
		}, {
			Name: "Filter Deployment.Update error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions),
				createChannel(testNS, triggerChannel, true),
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, "some-other-image", livenessProbe(), readinessProbe(), envVars(filterContainerName), containerPorts(8080))),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "deployments"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions,
					WithTriggerChannelReady(),
					WithBrokerTriggerChannel(createTriggerChannelRef()),
					WithFilterFailed("DeploymentFailure", "inducing failure for update deployments")),
			}},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, livenessProbe(), readinessProbe(), envVars(filterContainerName), containerPorts(8080))),
			}},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for update deployments"),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantErr: true,
		}, {
			Name: "Filter Service.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions),
				createChannel(testNS, triggerChannel, true),
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, livenessProbe(), readinessProbe(), envVars(filterContainerName), containerPorts(8080))),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "services"),
			},
			WantCreates: []runtime.Object{
				NewService(filterServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.FilterLabels(brokerName)),
					WithServicePorts(servicePorts(8080))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions,
					WithTriggerChannelReady(),
					WithBrokerTriggerChannel(createTriggerChannelRef()),
					WithFilterFailed("ServiceFailure", "inducing failure for create services")),
			}},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for create services"),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantErr: true,
		}, {
			Name: "Filter Service.Update error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions),
				createChannel(testNS, triggerChannel, true),
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, livenessProbe(), readinessProbe(), envVars(filterContainerName), containerPorts(8080))),
				NewService(filterServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.FilterLabels(brokerName)),
					WithServicePorts(servicePorts(9090))),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "services"),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewService(filterServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.FilterLabels(brokerName)),
					WithServicePorts(servicePorts(8080))),
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions,
					WithTriggerChannelReady(),
					WithBrokerTriggerChannel(createTriggerChannelRef()),
					WithFilterFailed("ServiceFailure", "inducing failure for update services")),
			}},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for update services"),
			},
			WantErr: true,
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
		}, {
			Name: "Ingress Deployment.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions),
				createChannel(testNS, triggerChannel, true),
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, livenessProbe(), readinessProbe(), envVars(filterContainerName), containerPorts(8080))),
				NewService(filterServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.FilterLabels(brokerName)),
					WithServicePorts(servicePorts(8080))),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "deployments"),
			},
			WantCreates: []runtime.Object{
				NewDeployment(ingressDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.IngressLabels(brokerName)),
					WithDeploymentServiceAccount(ingressSA),
					WithDeploymentContainer(ingressContainerName, ingressImage, livenessProbe(), nil, envVars(ingressContainerName), containerPorts(8080)),
				),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions,
					WithTriggerChannelReady(),
					WithFilterDeploymentAvailable(),
					WithBrokerTriggerChannel(createTriggerChannelRef()),
					WithIngressFailed("DeploymentFailure", "inducing failure for create deployments")),
			}},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for create deployments"),
			},
			WantErr: true,
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
		}, {
			Name: "Ingress Deployment.Update error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions,
					WithBrokerGeneration(brokerGeneration),
				),
				createChannel(testNS, triggerChannel, true),
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, livenessProbe(), readinessProbe(), envVars(filterContainerName), containerPorts(8080))),
				NewService(filterServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.FilterLabels(brokerName)),
					WithServicePorts(servicePorts(8080))),
				NewDeployment(ingressDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.IngressLabels(brokerName)),
					WithDeploymentServiceAccount(ingressSA),
					WithDeploymentContainer(ingressContainerName, ingressImage, livenessProbe(), nil, envVars(ingressContainerName), containerPorts(9090))),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "deployments"),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewDeployment(ingressDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.IngressLabels(brokerName)),
					WithDeploymentServiceAccount(ingressSA),
					WithDeploymentContainer(ingressContainerName, ingressImage, livenessProbe(), nil, envVars(ingressContainerName), containerPorts(8080))),
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions,
					WithBrokerGeneration(brokerGeneration),
					WithBrokerStatusObservedGeneration(brokerGeneration),
					WithTriggerChannelReady(),
					WithFilterDeploymentAvailable(),
					WithBrokerTriggerChannel(createTriggerChannelRef()),
					WithIngressFailed("DeploymentFailure", "inducing failure for update deployments")),
			}},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for update deployments"),
			},
			WantErr: true,
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
		}, {
			Name: "Ingress Service.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions),
				createChannel(testNS, triggerChannel, true),
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, livenessProbe(), readinessProbe(), envVars(filterContainerName), containerPorts(8080))),
				NewService(filterServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.FilterLabels(brokerName)),
					WithServicePorts(servicePorts(8080))),
				NewDeployment(ingressDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.IngressLabels(brokerName)),
					WithDeploymentServiceAccount(ingressSA),
					WithDeploymentContainer(ingressContainerName, ingressImage, livenessProbe(), nil, envVars(ingressContainerName), containerPorts(8080))),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "services"),
			},
			WantCreates: []runtime.Object{
				NewService(ingressServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.IngressLabels(brokerName)),
					WithServicePorts(servicePorts(8080))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions,
					WithTriggerChannelReady(),
					WithFilterDeploymentAvailable(),
					WithBrokerTriggerChannel(createTriggerChannelRef()),
					WithIngressFailed("ServiceFailure", "inducing failure for create services")),
			}},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for create services"),
			},
			WantErr: true,
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
		}, {
			Name: "Ingress Service.Update error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions),
				createChannel(testNS, triggerChannel, true),
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, livenessProbe(), readinessProbe(), envVars(filterContainerName), containerPorts(8080))),
				NewService(filterServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.FilterLabels(brokerName)),
					WithServicePorts(servicePorts(8080))),
				NewDeployment(ingressDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.IngressLabels(brokerName)),
					WithDeploymentServiceAccount(ingressSA),
					WithDeploymentContainer(ingressContainerName, ingressImage, livenessProbe(), nil, envVars(ingressContainerName), containerPorts(8080))),
				NewService(ingressServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.IngressLabels(brokerName)),
					WithServicePorts(servicePorts(9090))),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "services"),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewService(ingressServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.IngressLabels(brokerName)),
					WithServicePorts(servicePorts(8080))),
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions,
					WithTriggerChannelReady(),
					WithFilterDeploymentAvailable(),
					WithBrokerTriggerChannel(createTriggerChannelRef()),
					WithIngressFailed("ServiceFailure", "inducing failure for update services")),
			}},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for update services"),
			},
			WantErr: true,
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
		}, {
			Name: "Successful Reconciliation",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions),
				createChannel(testNS, triggerChannel, true),
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, livenessProbe(), readinessProbe(), envVars(filterContainerName), containerPorts(8080))),
				NewService(filterServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.FilterLabels(brokerName)),
					WithServicePorts(servicePorts(8080))),
				NewDeployment(ingressDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.IngressLabels(brokerName)),
					WithDeploymentServiceAccount(ingressSA),
					WithDeploymentContainer(ingressContainerName, ingressImage, livenessProbe(), nil, envVars(ingressContainerName), containerPorts(8080))),
				NewService(ingressServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.IngressLabels(brokerName)),
					WithServicePorts(servicePorts(8080))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithBrokerReady,
					WithBrokerTriggerChannel(createTriggerChannelRef()),
					WithBrokerAddress(fmt.Sprintf("%s.%s.svc.%s", ingressServiceName, testNS, utils.GetClusterDomainName())),
				),
			}},
			WantEvents: []string{
				finalizerUpdatedEvent,
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
		},
		{
			Name: "Successful Reconciliation, broker ignored because mismatching BrokerClass",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerChannel(channel()),
					WithInitBrokerConditions,
					WithBrokerClass("broker-class-mismatch")),
			},
		},
		{
			Name: "Successful Reconciliation, status update fails",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions),
				createChannel(testNS, triggerChannel, true),
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, livenessProbe(), readinessProbe(), envVars(filterContainerName), containerPorts(8080))),
				NewService(filterServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.FilterLabels(brokerName)),
					WithServicePorts(servicePorts(8080))),
				NewDeployment(ingressDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.IngressLabels(brokerName)),
					WithDeploymentServiceAccount(ingressSA),
					WithDeploymentContainer(ingressContainerName, ingressImage, livenessProbe(), nil, envVars(ingressContainerName), containerPorts(8080))),
				NewService(ingressServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.IngressLabels(brokerName)),
					WithServicePorts(servicePorts(8080))),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "brokers"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithBrokerReady,
					WithBrokerTriggerChannel(createTriggerChannelRef()),
					WithBrokerAddress(fmt.Sprintf("%s.%s.svc.%s", ingressServiceName, testNS, utils.GetClusterDomainName())),
				),
			}},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeWarning, "UpdateFailed", `Failed to update status for "test-broker": inducing failure for update brokers`),
			},
			WantErr: true,
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
		}, {
			Name: "Successful Reconciliation, with single trigger",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions),
				createChannel(testNS, triggerChannel, true),
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, livenessProbe(), readinessProbe(), envVars(filterContainerName), containerPorts(8080))),
				NewService(filterServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.FilterLabels(brokerName)),
					WithServicePorts(servicePorts(8080))),
				NewDeployment(ingressDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.IngressLabels(brokerName)),
					WithDeploymentServiceAccount(ingressSA),
					WithDeploymentContainer(ingressContainerName, ingressImage, livenessProbe(), nil, envVars(ingressContainerName), containerPorts(8080))),
				NewService(ingressServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.IngressLabels(brokerName)),
					WithServicePorts(servicePorts(8080))),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
			},
			WantCreates: []runtime.Object{
				makeIngressSubscription(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerBrokerReady(),
					WithTriggerDependencyReady(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerSubscribedUnknown("SubscriptionNotConfigured", "Subscription has not yet been reconciled."),
					WithTriggerStatusSubscriberURI(subscriberURI)),
			}, {
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithBrokerReady,
					WithBrokerTriggerChannel(createTriggerChannelRef()),
					WithBrokerAddress(fmt.Sprintf("%s.%s.svc.%s", ingressServiceName, testNS, utils.GetClusterDomainName())))},
			},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
		}, {
			Name: "Fail Reconciliation, with single trigger, trigger status updated",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithInitBrokerConditions),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerDependencyReady(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerSubscribedUnknown("SubscriptionNotConfigured", "Subscription has not yet been reconciled."),
					WithTriggerStatusSubscriberURI(subscriberURI)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerDependencyReady(),
					WithTriggerSubscribedUnknown("SubscriptionNotConfigured", "Subscription has not yet been reconciled."),
					WithTriggerBrokerFailed("ChannelTemplateFailed", "Error on setting up the ChannelTemplate: Broker.Spec.ChannelTemplate is nil"),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerStatusSubscriberURI(subscriberURI)),
			}, {
				Object: NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithInitBrokerConditions,
					WithTriggerChannelFailed("ChannelTemplateFailed", "Error on setting up the ChannelTemplate: Broker.Spec.ChannelTemplate is nil")),
			}},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeWarning, "InternalError", "Broker.Spec.ChannelTemplate is nil"),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, brokerName),
			},
			WantErr: true,
		}, {
			Name: "Broker being deleted, marks trigger as not ready due to broker missing",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions,
					WithBrokerFinalizers("brokers.eventing.knative.dev"),
					WithBrokerResourceVersion(""),
					WithBrokerDeletionTimestamp),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerBrokerFailed("BrokerDoesNotExist", `Broker "test-broker" does not exist`)),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchRemoveFinalizers(testNS, brokerName),
			},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeNormal, "BrokerReconciled", `Broker reconciled: "test-namespace/test-broker"`),
			},
		}, {
			Name: "Broker being deleted, marks trigger as not ready due to broker missing, fails",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerClass(eventing.ChannelBrokerClassValue),
					WithBrokerChannel(channel()),
					WithInitBrokerConditions,
					WithBrokerFinalizers("brokers.eventing.knative.dev"),
					WithBrokerResourceVersion(""),
					WithBrokerDeletionTimestamp),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "triggers"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerBrokerFailed("BrokerDoesNotExist", `Broker "test-broker" does not exist`)),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "TriggerUpdateStatusFailed", `Failed to update Trigger's status: inducing failure for update triggers`),
				Eventf(corev1.EventTypeWarning, "InternalError", "Trigger reconcile failed: inducing failure for update triggers"),
			},
			WantErr: true,
		}, {
			Name: "Trigger being deleted",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerDeleted,
					WithTriggerSubscriberURI(subscriberURI))}...),
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerDeleted,
					WithInitTriggerConditions,
					WithTriggerSubscriberURI(subscriberURI)),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
		}, {
			Name: "Trigger subscription create fails",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI))}...),
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "subscriptions"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerNotSubscribed("NotSubscribed", "inducing failure for create subscriptions")),
			}},
			WantCreates: []runtime.Object{
				makeIngressSubscription(),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriptionCreateFailed", "Create Trigger's subscription failed: inducing failure for create subscriptions"),
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconcile failed: inducing failure for create subscriptions"),
			},
		}, {
			Name: "Trigger subscription create fails, update status fails",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI))}...),
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "subscriptions"),
				InduceFailure("update", "triggers"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerNotSubscribed("NotSubscribed", "inducing failure for create subscriptions")),
			}},
			WantCreates: []runtime.Object{
				makeIngressSubscription(),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriptionCreateFailed", "Create Trigger's subscription failed: inducing failure for create subscriptions"),
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconcile failed: inducing failure for create subscriptions"),
				Eventf(corev1.EventTypeWarning, "TriggerUpdateStatusFailed", "Failed to update Trigger's status: inducing failure for update triggers"),
			},
		}, {
			Name: "Trigger subscription delete fails",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
				makeDifferentReadySubscription()}...),
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("delete", "subscriptions"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerNotSubscribed("NotSubscribed", "inducing failure for delete subscriptions"))},
			},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				Name: subscriptionName,
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriptionDeleteFailed", "Delete Trigger's subscription failed: inducing failure for delete subscriptions"),
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconcile failed: inducing failure for delete subscriptions"),
			},
		}, {
			Name: "Trigger subscription create after delete fails",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
				makeDifferentReadySubscription()}...),
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "subscriptions"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerNotSubscribed("NotSubscribed", "inducing failure for create subscriptions")),
			}},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				Name: subscriptionName,
			}},
			WantCreates: []runtime.Object{
				makeIngressSubscription(),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriptionCreateFailed", "Create Trigger's subscription failed: inducing failure for create subscriptions"),
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconcile failed: inducing failure for create subscriptions"),
			},
		}, {
			Name: "Trigger subscription not owned by Trigger",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
				makeIngressSubscriptionNotOwnedByTrigger()}...),
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerSubscriberURI(subscriberURI),
					WithTriggerUID(triggerUID),
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerNotSubscribed("NotSubscribed", `trigger "test-trigger" does not own subscription "test-broker-test-trigger-test-trigger-uid"`),
					WithTriggerStatusSubscriberURI(subscriberURI)),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", `Trigger reconcile failed: trigger "test-trigger" does not own subscription "test-broker-test-trigger-test-trigger-uid"`),
			},
		}, {
			Name: "Trigger subscription update works",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI)),
				makeDifferentReadySubscription()}...),
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriptionNotConfigured(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyReady()),
			}},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				Name: subscriptionName,
			}},
			WantCreates: []runtime.Object{
				makeIngressSubscription(),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
		}, {
			Name: "Trigger has subscriber ref exists",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeSubscriberAddressableAsUnstructured(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithInitTriggerConditions)}...),
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriptionNotConfigured(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyReady(),
				),
			}},
			WantCreates: []runtime.Object{
				makeIngressSubscription(),
			},
		}, {
			Name: "Trigger has subscriber ref exists and URI",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeSubscriberAddressableAsUnstructured(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRefAndURIReference(subscriberGVK, subscriberName, testNS, subscriberURIReference),
					WithInitTriggerConditions,
				)}...),
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRefAndURIReference(subscriberGVK, subscriberName, testNS, subscriberURIReference),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriptionNotConfigured(),
					WithTriggerStatusSubscriberURI(subscriberResolvedTargetURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyReady(),
				),
			}},
			WantCreates: []runtime.Object{
				makeIngressSubscription(),
			},
		}, {
			Name: "Trigger has subscriber ref exists kubernetes Service",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeSubscriberKubernetesServiceAsUnstructured(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(k8sServiceGVK, subscriberName, testNS),
					WithInitTriggerConditions,
				)}...),
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(k8sServiceGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriptionNotConfigured(),
					WithTriggerStatusSubscriberURI(k8sServiceResolvedURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyReady(),
				),
			}},
			WantCreates: []runtime.Object{
				makeIngressSubscription(),
			},
		}, {
			Name: "Trigger has subscriber ref doesn't exist",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					WithInitTriggerConditions,
				)}...),
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", `Trigger reconcile failed: failed to get ref &ObjectReference{Kind:Service,Namespace:test-namespace,Name:subscriber-name,UID:,APIVersion:serving.knative.dev/v1,ResourceVersion:,FieldPath:,}: services.serving.knative.dev "subscriber-name" not found`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberRef(subscriberGVK, subscriberName, testNS),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscriberResolvedFailed("Unable to get the Subscriber's URI", `failed to get ref &ObjectReference{Kind:Service,Namespace:test-namespace,Name:subscriber-name,UID:,APIVersion:serving.knative.dev/v1,ResourceVersion:,FieldPath:,}: services.serving.knative.dev "subscriber-name" not found`),
				),
			}},
		}, {
			Name: "Subscription not ready, trigger marked not ready",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeFalseStatusSubscription(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
				)}...),
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerNotSubscribed("testInducedError", "test induced error"),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyReady(),
				),
			}},
		}, {
			Name: "Subscription ready, trigger marked ready",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeReadySubscription(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
				)}...),
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
				Eventf(corev1.EventTypeNormal, "TriggerReadinessChanged", `Trigger "test-trigger" became ready`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithTriggerBrokerReady(),
					WithTriggerSubscribed(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyReady(),
				),
			}},
		}, {
			Name: "Dependency doesn't exist",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeReadySubscription(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
				)}...),
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconcile failed: propagating dependency readiness: getting the dependency: pingsources.sources.knative.dev \"test-ping-source\" not found"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
					WithTriggerBrokerReady(),
					WithTriggerSubscribed(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyFailed("DependencyDoesNotExist", "Dependency does not exist: pingsources.sources.knative.dev \"test-ping-source\" not found"),
				),
			}},
		}, {
			Name: "The status of Dependency is False",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeReadySubscription(),
				makeFalseStatusPingSource(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
				)}...),
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled")},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
					WithTriggerBrokerReady(),
					WithTriggerSubscribed(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyFailed("NotFound", ""),
				),
			}},
		}, {
			Name: "The status of Dependency is Unknown",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeReadySubscription(),
				makeUnknownStatusCronJobSource(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
				)}...),
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled")},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
					WithTriggerBrokerReady(),
					WithTriggerSubscribed(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyUnknown("", ""),
				),
			}},
		},
		{
			Name: "Dependency generation not equal",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeReadySubscription(),
				makeGenerationNotEqualPingSource(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
				)}...),
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled")},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
					WithTriggerBrokerReady(),
					WithTriggerSubscribed(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyUnknown("GenerationNotEqual", fmt.Sprintf("The dependency's metadata.generation, %q, is not equal to its status.observedGeneration, %q.", currentGeneration, outdatedGeneration))),
			}},
		},
		{
			Name: "Dependency ready",
			Key:  testKey,
			Objects: allBrokerObjectsReadyPlus([]runtime.Object{
				makeReadySubscription(),
				makeReadyPingSource(),
				NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
				)}...),
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
				Eventf(corev1.EventTypeNormal, "TriggerReadinessChanged", `Trigger "test-trigger" became ready`)},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewTrigger(triggerName, testNS, brokerName,
					WithTriggerUID(triggerUID),
					WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					WithInitTriggerConditions,
					WithDependencyAnnotation(dependencyAnnotation),
					WithTriggerBrokerReady(),
					WithTriggerSubscribed(),
					WithTriggerStatusSubscriberURI(subscriberURI),
					WithTriggerSubscriberResolvedSucceeded(),
					WithTriggerDependencyReady(),
				),
			}},
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = channelable.WithDuck(ctx)
		ctx = v1a1addr.WithDuck(ctx)
		ctx = v1b1addr.WithDuck(ctx)
		ctx = v1addr.WithDuck(ctx)
		ctx = conditions.WithDuck(ctx)
		r := &Reconciler{
			Base:                      reconciler.NewBase(ctx, controllerAgentName, cmw),
			subscriptionLister:        listers.GetSubscriptionLister(),
			triggerLister:             listers.GetTriggerLister(),
			brokerLister:              listers.GetBrokerLister(),
			serviceLister:             listers.GetK8sServiceLister(),
			deploymentLister:          listers.GetDeploymentLister(),
			filterImage:               filterImage,
			filterServiceAccountName:  filterSA,
			ingressImage:              ingressImage,
			ingressServiceAccountName: ingressSA,
			kresourceTracker:          duck.NewListableTracker(ctx, conditions.Get, func(types.NamespacedName) {}, 0),
			channelableTracker:        duck.NewListableTracker(ctx, channelable.Get, func(types.NamespacedName) {}, 0),
			addressableTracker:        duck.NewListableTracker(ctx, v1a1addr.Get, func(types.NamespacedName) {}, 0),
			uriResolver:               resolver.NewURIResolver(ctx, func(types.NamespacedName) {}),
			brokerClass:               eventing.ChannelBrokerClassValue,
		}
		return broker.NewReconciler(ctx, r.Logger, r.EventingClientSet, listers.GetBrokerLister(), r.Recorder, r, eventing.ChannelBrokerClassValue)

	},
		false,
		logger,
	))
}

func ownerReferences() []metav1.OwnerReference {
	return []metav1.OwnerReference{{
		APIVersion:         v1alpha1.SchemeGroupVersion.String(),
		Kind:               "Broker",
		Name:               brokerName,
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
	}}
}

func channel() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: "messaging.knative.dev/v1alpha1",
		Kind:       "InMemoryChannel",
	}
}

func livenessProbe() *corev1.Probe {
	return &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/healthz",
				Port: intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
			},
		},
		InitialDelaySeconds: 5,
		PeriodSeconds:       2,
	}
}

func readinessProbe() *corev1.Probe {
	return &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/readyz",
				Port: intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
			},
		},
		InitialDelaySeconds: 5,
		PeriodSeconds:       2,
	}
}

func envVars(containerName string) []corev1.EnvVar {
	switch containerName {
	case filterContainerName:
		return []corev1.EnvVar{
			{
				Name:  system.NamespaceEnvKey,
				Value: system.Namespace(),
			},
			{
				Name: "NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
			{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
			{
				Name:  "CONTAINER_NAME",
				Value: filterContainerName,
			},
			{
				Name:  "BROKER",
				Value: brokerName,
			},
			{
				Name:  "METRICS_DOMAIN",
				Value: "knative.dev/internal/eventing",
			},
		}
	case ingressContainerName:
		return []corev1.EnvVar{
			{
				Name:  system.NamespaceEnvKey,
				Value: system.Namespace(),
			},
			{
				Name: "NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
			{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
			{
				Name:  "CONTAINER_NAME",
				Value: ingressContainerName,
			},
			{
				Name:  "FILTER",
				Value: "",
			},
			{
				Name:  "CHANNEL",
				Value: triggerChannelHostname,
			},
			{
				Name:  "BROKER",
				Value: brokerName,
			},
			{
				Name:  "METRICS_DOMAIN",
				Value: "knative.dev/internal/eventing",
			},
		}
	}
	return []corev1.EnvVar{}
}

func containerPorts(httpInternal int32) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          "http",
			ContainerPort: httpInternal,
		},
		{
			Name:          "metrics",
			ContainerPort: 9090,
		},
	}
}

func servicePorts(httpInternal int) []corev1.ServicePort {
	svcPorts := []corev1.ServicePort{
		{
			Name:       "http",
			Port:       80,
			TargetPort: intstr.FromInt(httpInternal),
		}, {
			Name: "http-metrics",
			Port: 9090,
		},
	}
	return svcPorts
}

func createChannel(namespace string, t channelType, ready bool) *unstructured.Unstructured {
	var labels map[string]interface{}
	var name string
	var hostname string
	var url string
	if t == triggerChannel {
		name = fmt.Sprintf("%s-kne-trigger", brokerName)
		labels = map[string]interface{}{
			eventing.BrokerLabelKey:                 brokerName,
			"eventing.knative.dev/brokerEverything": "true",
		}
		hostname = triggerChannelHostname
		url = fmt.Sprintf("http://%s", triggerChannelHostname)
	}
	if ready {
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "messaging.knative.dev/v1alpha1",
				"kind":       "InMemoryChannel",
				"metadata": map[string]interface{}{
					"creationTimestamp": nil,
					"namespace":         namespace,
					"name":              name,
					"ownerReferences": []interface{}{
						map[string]interface{}{
							"apiVersion":         "eventing.knative.dev/v1alpha1",
							"blockOwnerDeletion": true,
							"controller":         true,
							"kind":               "Broker",
							"name":               brokerName,
							"uid":                "",
						},
					},
					"labels": labels,
				},
				"status": map[string]interface{}{
					"address": map[string]interface{}{
						"hostname": hostname,
						"url":      url,
					},
				},
			},
		}
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "messaging.knative.dev/v1alpha1",
			"kind":       "InMemoryChannel",
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"namespace":         namespace,
				"name":              name,
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "eventing.knative.dev/v1alpha1",
						"blockOwnerDeletion": true,
						"controller":         true,
						"kind":               "Broker",
						"name":               brokerName,
						"uid":                "",
					},
				},
				"labels": labels,
			},
		},
	}
}

func createTriggerChannelRef() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: "messaging.knative.dev/v1alpha1",
		Kind:       "InMemoryChannel",
		Namespace:  testNS,
		Name:       fmt.Sprintf("%s-kne-trigger", brokerName),
	}
}

func makeIngressSubscription() *messagingv1alpha1.Subscription {
	return resources.NewSubscription(makeTrigger(), createTriggerChannelRef(), makeBrokerRef(), makeServiceURI(), makeEmptyDelivery())
}

func makeTrigger() *v1alpha1.Trigger {
	return &v1alpha1.Trigger{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "eventing.knative.dev/v1alpha1",
			Kind:       "Trigger",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      triggerName,
			UID:       triggerUID,
		},
		Spec: v1alpha1.TriggerSpec{
			Broker: brokerName,
			Filter: &v1alpha1.TriggerFilter{
				DeprecatedSourceAndType: &v1alpha1.TriggerFilterSourceAndType{
					Source: "Any",
					Type:   "Any",
				},
			},
			Subscriber: duckv1.Destination{
				Ref: &duckv1.KReference{
					Name:       subscriberName,
					Namespace:  testNS,
					Kind:       subscriberKind,
					APIVersion: subscriberAPIVersion,
				},
			},
		},
	}
}

func makeBrokerRef() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: "eventing.knative.dev/v1alpha1",
		Kind:       "Broker",
		Namespace:  testNS,
		Name:       brokerName,
	}
}
func makeServiceURI() *apis.URL {
	return &apis.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s.%s.svc.%s", makeBrokerFilterService().Name, testNS, utils.GetClusterDomainName()),
		Path:   fmt.Sprintf("/triggers/%s/%s/%s", testNS, triggerName, triggerUID),
	}
}
func makeEmptyDelivery() *eventingduckv1beta1.DeliverySpec {
	return nil
}
func makeBrokerFilterService() *corev1.Service {
	return resources.MakeFilterService(makeBroker())
}

func makeBroker() *v1alpha1.Broker {
	return &v1alpha1.Broker{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "eventing.knative.dev/v1alpha1",
			Kind:       "Broker",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      brokerName,
		},
		Spec: v1alpha1.BrokerSpec{},
	}
}

func allBrokerObjectsReadyPlus(objs ...runtime.Object) []runtime.Object {
	brokerObjs := []runtime.Object{
		NewBroker(brokerName, testNS,
			WithBrokerClass(eventing.ChannelBrokerClassValue),
			WithBrokerChannel(channel()),
			WithInitBrokerConditions,
			WithBrokerReady,
			WithBrokerFinalizers("brokers.eventing.knative.dev"),
			WithBrokerResourceVersion(""),
			WithBrokerTriggerChannel(createTriggerChannelRef()),
			WithBrokerAddress(fmt.Sprintf("%s.%s.svc.%s", ingressServiceName, testNS, utils.GetClusterDomainName()))),
		createChannel(testNS, triggerChannel, true),
		NewDeployment(filterDeploymentName, testNS,
			WithDeploymentOwnerReferences(ownerReferences()),
			WithDeploymentLabels(resources.FilterLabels(brokerName)),
			WithDeploymentServiceAccount(filterSA),
			WithDeploymentContainer(filterContainerName, filterImage, livenessProbe(), readinessProbe(), envVars(filterContainerName), containerPorts(8080))),
		NewService(filterServiceName, testNS,
			WithServiceOwnerReferences(ownerReferences()),
			WithServiceLabels(resources.FilterLabels(brokerName)),
			WithServicePorts(servicePorts(8080))),
		NewDeployment(ingressDeploymentName, testNS,
			WithDeploymentOwnerReferences(ownerReferences()),
			WithDeploymentLabels(resources.IngressLabels(brokerName)),
			WithDeploymentServiceAccount(ingressSA),
			WithDeploymentContainer(ingressContainerName, ingressImage, livenessProbe(), nil, envVars(ingressContainerName), containerPorts(8080))),
		NewService(ingressServiceName, testNS,
			WithServiceOwnerReferences(ownerReferences()),
			WithServiceLabels(resources.IngressLabels(brokerName)),
			WithServicePorts(servicePorts(8080))),
	}
	return append(brokerObjs[:], objs...)
}

// Just so we can test subscription updates
func makeDifferentReadySubscription() *messagingv1alpha1.Subscription {
	s := makeIngressSubscription()
	s.Spec.Subscriber.URI = apis.HTTP("different.example.com")
	s.Status = *v1alpha1.TestHelper.ReadySubscriptionStatus()
	return s
}

func makeIngressSubscriptionNotOwnedByTrigger() *messagingv1alpha1.Subscription {
	sub := makeIngressSubscription()
	sub.OwnerReferences = []metav1.OwnerReference{}
	return sub
}

func makeReadySubscription() *messagingv1alpha1.Subscription {
	s := makeIngressSubscription()
	s.Status = *v1alpha1.TestHelper.ReadySubscriptionStatus()
	return s
}

func makeSubscriberAddressableAsUnstructured() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": subscriberAPIVersion,
			"kind":       subscriberKind,
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      subscriberName,
			},
			"status": map[string]interface{}{
				"address": map[string]interface{}{
					"url": subscriberURI,
				},
			},
		},
	}
}

func makeFalseStatusSubscription() *messagingv1alpha1.Subscription {
	s := makeIngressSubscription()
	s.Status = *v1alpha1.TestHelper.FalseSubscriptionStatus()
	return s
}

func makeFalseStatusPingSource() *sourcesv1alpha2.PingSource {
	return NewPingSourceV1Alpha2(pingSourceName, testNS, WithPingSourceV1A2SinkNotFound)
}

func makeUnknownStatusCronJobSource() *sourcesv1alpha2.PingSource {
	cjs := NewPingSourceV1Alpha2(pingSourceName, testNS)
	cjs.Status.InitializeConditions()
	return cjs
}

func makeGenerationNotEqualPingSource() *sourcesv1alpha2.PingSource {
	c := makeFalseStatusPingSource()
	c.Generation = currentGeneration
	c.Status.ObservedGeneration = outdatedGeneration
	return c
}

func makeReadyPingSource() *sourcesv1alpha2.PingSource {
	u, _ := apis.ParseURL(sinkURI)
	return NewPingSourceV1Alpha2(pingSourceName, testNS,
		WithPingSourceV1A2Spec(sourcesv1alpha2.PingSourceSpec{
			Schedule: testSchedule,
			JsonData: testData,
			SourceSpec: duckv1.SourceSpec{
				Sink: brokerDestv1,
			},
		}),
		WithInitPingSourceV1A2Conditions,
		WithValidPingSourceV1A2Schedule,
		WithValidPingSourceV1A2Resources,
		WithPingSourceV1A2Deployed,
		WithPingSourceV1A2EventType,
		WithPingSourceV1A2Sink(u),
	)
}
func makeSubscriberKubernetesServiceAsUnstructured() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      subscriberName,
			},
		},
	}
}

func patchFinalizers(namespace, name string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	patch := `{"metadata":{"finalizers":["` + finalizerName + `"],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}

func patchRemoveFinalizers(namespace, name string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	patch := `{"metadata":{"finalizers":[],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}
