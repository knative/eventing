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

package channel

import (
	"context"
	"fmt"
	"testing"

	"github.com/knative/eventing/pkg/apis/messaging/v1alpha1"
	"github.com/knative/eventing/pkg/reconciler"
	. "github.com/knative/eventing/pkg/reconciler/testing"
	"github.com/knative/eventing/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	testNS      = "test-namespace"
	channelName = "test-channel"
)

var (
	trueVal = true

	testKey = fmt.Sprintf("%s/%s", testNS, channelName)

	backingChannelHostname = fmt.Sprintf("foo.bar.svc.%s", utils.GetClusterDomainName())

	channelGVK = metav1.GroupVersionKind{
		Group:   "messaging.knative.dev",
		Version: "v1alpha1",
		Kind:    "Channel",
	}

	imcGVK = metav1.GroupVersionKind{
		Group:   "messaging.knative.dev",
		Version: "v1alpha1",
		Kind:    "InMemoryChannel",
	}
)

func init() {
	// Add types to scheme
	_ = v1alpha1.AddToScheme(scheme.Scheme)
}

type fakeResourceTracker struct{}

func (fakeResourceTracker) TrackInNamespace(metav1.Object) func(corev1.ObjectReference) error {
	return func(corev1.ObjectReference) error { return nil }
}

func (fakeResourceTracker) Track(ref corev1.ObjectReference, obj interface{}) error {
	return nil
}

func (fakeResourceTracker) OnChanged(obj interface{}) {
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
		},
		{
			Name: "Channel not found",
			Key:  testKey,
		},
		{
			Name: "Channel is being deleted",
			Key:  testKey,
			Objects: []runtime.Object{
				NewMessagingChannel(channelName, testNS,
					WithMessagingChannelTemplate(channelCRD()),
					WithInitMessagingChannelConditions,
					WithMessagingChannelDeleted),
			},
		},
		{
			Name: "Backing Channel.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewMessagingChannel(channelName, testNS,
					WithMessagingChannelTemplate(channelCRD()),
					WithInitMessagingChannelConditions),
			},
			WantCreates: []runtime.Object{
				createChannelCRD(testNS, channelName, false),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewMessagingChannel(channelName, testNS,
					WithInitMessagingChannelConditions,
					WithMessagingChannelTemplate(channelCRD()),
					WithBackingChannelFailed("ChannelFailure", "inducing failure for create inmemorychannels")),
			}},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "inmemorychannels"),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, channelReconcileError, "Channel reconcile error: %v", "inducing failure for create inmemorychannels"),
			},
			WantErr: true,
		},
		//{
		//	Name: "Trigger Channel.Create no address",
		//	Key:  testKey,
		//	Objects: []runtime.Object{
		//		NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions),
		//	},
		//	WantCreates: []runtime.Object{
		//		createChannelCRD(testNS, triggerChannel, false),
		//	},
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewBroker(brokerName, testNS,
		//			WithInitBrokerConditions,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithTriggerChannelFailed("NoAddress", "Channel does not have an address.")),
		//	}},
		//},
		//{
		//	Name: "Trigger Channel is not yet Addressable",
		//	Key:  testKey,
		//	Objects: []runtime.Object{
		//		NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions),
		//		createChannelCRD(testNS, triggerChannel, false),
		//	},
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions,
		//			WithTriggerChannelFailed("NoAddress", "Channel does not have an address.")),
		//	}},
		//},
		//{
		//	Name: "Filter Deployment.Create error",
		//	Key:  testKey,
		//	Objects: []runtime.Object{
		//		NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions),
		//		createChannelCRD(testNS, triggerChannel, true),
		//	},
		//	WithReactors: []clientgotesting.ReactionFunc{
		//		InduceFailure("create", "deployments"),
		//	},
		//	WantCreates: []runtime.Object{
		//		NewDeployment(filterDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.FilterLabels(brokerName)),
		//			WithDeploymentServiceAccount(filterSA),
		//			WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), containerPorts(8080))),
		//	},
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions,
		//			WithTriggerChannelReady(),
		//			WithBrokerTriggerChannel(createTriggerChannelCRDRef()),
		//			WithFilterFailed("DeploymentFailure", "inducing failure for create deployments")),
		//	}},
		//	WantEvents: []string{
		//		Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for create deployments"),
		//	},
		//	WantErr: true,
		//},
		//{
		//	Name: "Filter Deployment.Update error",
		//	Key:  testKey,
		//	Objects: []runtime.Object{
		//		NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions),
		//		createChannelCRD(testNS, triggerChannel, true),
		//		NewDeployment(filterDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.FilterLabels(brokerName)),
		//			WithDeploymentServiceAccount(filterSA),
		//			WithDeploymentContainer(filterContainerName, "some-other-image", envVars(filterContainerName), containerPorts(8080))),
		//	},
		//	WithReactors: []clientgotesting.ReactionFunc{
		//		InduceFailure("update", "deployments"),
		//	},
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions,
		//			WithTriggerChannelReady(),
		//			WithBrokerTriggerChannel(createTriggerChannelCRDRef()),
		//			WithFilterFailed("DeploymentFailure", "inducing failure for update deployments")),
		//	}},
		//	WantUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewDeployment(filterDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.FilterLabels(brokerName)),
		//			WithDeploymentServiceAccount(filterSA),
		//			WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), containerPorts(8080))),
		//	}},
		//	WantEvents: []string{
		//		Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for update deployments"),
		//	},
		//	WantErr: true,
		//},
		//{
		//	Name: "Filter Service.Create error",
		//	Key:  testKey,
		//	Objects: []runtime.Object{
		//		NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions),
		//		createChannelCRD(testNS, triggerChannel, true),
		//		NewDeployment(filterDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.FilterLabels(brokerName)),
		//			WithDeploymentServiceAccount(filterSA),
		//			WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), containerPorts(8080))),
		//	},
		//	WithReactors: []clientgotesting.ReactionFunc{
		//		InduceFailure("create", "services"),
		//	},
		//	WantCreates: []runtime.Object{
		//		NewService(filterServiceName, testNS,
		//			WithServiceOwnerReferences(ownerReferences()),
		//			WithServiceAnnotations(resources.FilterAnnotations()),
		//			WithServiceLabels(resources.FilterLabels(brokerName)),
		//			WithServicePorts(servicePorts(filterContainerName, 8080))),
		//	},
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions,
		//			WithTriggerChannelReady(),
		//			WithBrokerTriggerChannel(createTriggerChannelCRDRef()),
		//			WithFilterFailed("ServiceFailure", "inducing failure for create services")),
		//	}},
		//	WantEvents: []string{
		//		Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for create services"),
		//	},
		//	WantErr: true,
		//},
		//{
		//	Name: "Filter Service.Update error",
		//	Key:  testKey,
		//	Objects: []runtime.Object{
		//		NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions),
		//		createChannelCRD(testNS, triggerChannel, true),
		//		NewDeployment(filterDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.FilterLabels(brokerName)),
		//			WithDeploymentServiceAccount(filterSA),
		//			WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), containerPorts(8080))),
		//		NewService(filterServiceName, testNS,
		//			WithServiceOwnerReferences(ownerReferences()),
		//			WithServiceLabels(resources.FilterLabels(brokerName)),
		//			WithServicePorts(servicePorts(filterContainerName, 9090))),
		//	},
		//	WithReactors: []clientgotesting.ReactionFunc{
		//		InduceFailure("update", "services"),
		//	},
		//	WantUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewService(filterServiceName, testNS,
		//			WithServiceOwnerReferences(ownerReferences()),
		//			WithServiceLabels(resources.FilterLabels(brokerName)),
		//			WithServicePorts(servicePorts(filterContainerName, 8080))),
		//	}},
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions,
		//			WithTriggerChannelReady(),
		//			WithBrokerTriggerChannel(createTriggerChannelCRDRef()),
		//			WithFilterFailed("ServiceFailure", "inducing failure for update services")),
		//	}},
		//	WantEvents: []string{
		//		Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for update services"),
		//	},
		//	WantErr: true,
		//},
		//{
		//	Name: "Ingress Deployment.Create error",
		//	Key:  testKey,
		//	Objects: []runtime.Object{
		//		NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions),
		//		createChannelCRD(testNS, triggerChannel, true),
		//		NewDeployment(filterDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.FilterLabels(brokerName)),
		//			WithDeploymentServiceAccount(filterSA),
		//			WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), containerPorts(8080))),
		//		NewService(filterServiceName, testNS,
		//			WithServiceOwnerReferences(ownerReferences()),
		//			WithServiceLabels(resources.FilterLabels(brokerName)),
		//			WithServicePorts(servicePorts(filterContainerName, 8080))),
		//	},
		//	WithReactors: []clientgotesting.ReactionFunc{
		//		InduceFailure("create", "deployments"),
		//	},
		//	WantCreates: []runtime.Object{
		//		NewDeployment(ingressDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.IngressLabels(brokerName)),
		//			WithDeploymentServiceAccount(ingressSA),
		//			WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080)),
		//		),
		//	},
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions,
		//			WithTriggerChannelReady(),
		//			WithFilterDeploymentAvailable(),
		//			WithBrokerTriggerChannel(createTriggerChannelCRDRef()),
		//			WithIngressFailed("DeploymentFailure", "inducing failure for create deployments")),
		//	}},
		//	WantEvents: []string{
		//		Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for create deployments"),
		//	},
		//	WantErr: true,
		//},
		//{
		//	Name: "Ingress Deployment.Update error",
		//	Key:  testKey,
		//	Objects: []runtime.Object{
		//		NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions),
		//		createChannelCRD(testNS, triggerChannel, true),
		//		NewDeployment(filterDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.FilterLabels(brokerName)),
		//			WithDeploymentServiceAccount(filterSA),
		//			WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), containerPorts(8080))),
		//		NewService(filterServiceName, testNS,
		//			WithServiceOwnerReferences(ownerReferences()),
		//			WithServiceLabels(resources.FilterLabels(brokerName)),
		//			WithServicePorts(servicePorts(filterContainerName, 8080))),
		//		NewDeployment(ingressDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.IngressLabels(brokerName)),
		//			WithDeploymentServiceAccount(ingressSA),
		//			WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(9090))),
		//	},
		//	WithReactors: []clientgotesting.ReactionFunc{
		//		InduceFailure("update", "deployments"),
		//	},
		//	WantUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewDeployment(ingressDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.IngressLabels(brokerName)),
		//			WithDeploymentServiceAccount(ingressSA),
		//			WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080))),
		//	}},
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions,
		//			WithTriggerChannelReady(),
		//			WithFilterDeploymentAvailable(),
		//			WithBrokerTriggerChannel(createTriggerChannelCRDRef()),
		//			WithIngressFailed("DeploymentFailure", "inducing failure for update deployments")),
		//	}},
		//	WantEvents: []string{
		//		Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for update deployments"),
		//	},
		//	WantErr: true,
		//},
		//{
		//	Name: "Ingress Service.Create error",
		//	Key:  testKey,
		//	Objects: []runtime.Object{
		//		NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions),
		//		createChannelCRD(testNS, triggerChannel, true),
		//		NewDeployment(filterDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.FilterLabels(brokerName)),
		//			WithDeploymentServiceAccount(filterSA),
		//			WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), containerPorts(8080))),
		//		NewService(filterServiceName, testNS,
		//			WithServiceOwnerReferences(ownerReferences()),
		//			WithServiceLabels(resources.FilterLabels(brokerName)),
		//			WithServicePorts(servicePorts(filterContainerName, 8080))),
		//		NewDeployment(ingressDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.IngressLabels(brokerName)),
		//			WithDeploymentServiceAccount(ingressSA),
		//			WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080))),
		//	},
		//	WithReactors: []clientgotesting.ReactionFunc{
		//		InduceFailure("create", "services"),
		//	},
		//	WantCreates: []runtime.Object{
		//		NewService(ingressServiceName, testNS,
		//			WithServiceOwnerReferences(ownerReferences()),
		//			WithServiceAnnotations(resources.IngressAnnotations()),
		//			WithServiceLabels(resources.IngressLabels(brokerName)),
		//			WithServicePorts(servicePorts(ingressContainerName, 8080))),
		//	},
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions,
		//			WithTriggerChannelReady(),
		//			WithFilterDeploymentAvailable(),
		//			WithBrokerTriggerChannel(createTriggerChannelCRDRef()),
		//			WithIngressFailed("ServiceFailure", "inducing failure for create services")),
		//	}},
		//	WantEvents: []string{
		//		Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for create services"),
		//	},
		//	WantErr: true,
		//},
		//{
		//	Name: "Ingress Service.Update error",
		//	Key:  testKey,
		//	Objects: []runtime.Object{
		//		NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions),
		//		createChannelCRD(testNS, triggerChannel, true),
		//		NewDeployment(filterDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.FilterLabels(brokerName)),
		//			WithDeploymentServiceAccount(filterSA),
		//			WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), containerPorts(8080))),
		//		NewService(filterServiceName, testNS,
		//			WithServiceOwnerReferences(ownerReferences()),
		//			WithServiceLabels(resources.FilterLabels(brokerName)),
		//			WithServicePorts(servicePorts(filterContainerName, 8080))),
		//		NewDeployment(ingressDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.IngressLabels(brokerName)),
		//			WithDeploymentServiceAccount(ingressSA),
		//			WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080))),
		//		NewService(ingressServiceName, testNS,
		//			WithServiceOwnerReferences(ownerReferences()),
		//			WithServiceLabels(resources.IngressLabels(brokerName)),
		//			WithServicePorts(servicePorts(ingressContainerName, 9090))),
		//	},
		//	WithReactors: []clientgotesting.ReactionFunc{
		//		InduceFailure("update", "services"),
		//	},
		//	WantUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewService(ingressServiceName, testNS,
		//			WithServiceOwnerReferences(ownerReferences()),
		//			WithServiceLabels(resources.IngressLabels(brokerName)),
		//			WithServicePorts(servicePorts(ingressContainerName, 8080))),
		//	}},
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions,
		//			WithTriggerChannelReady(),
		//			WithFilterDeploymentAvailable(),
		//			WithBrokerTriggerChannel(createTriggerChannelCRDRef()),
		//			WithIngressFailed("ServiceFailure", "inducing failure for update services")),
		//	}},
		//	WantEvents: []string{
		//		Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for update services"),
		//	},
		//	WantErr: true,
		//},
		//{
		//	Name: "Ingress Channel.Create error",
		//	Key:  testKey,
		//	Objects: []runtime.Object{
		//		NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions),
		//		createChannelCRD(testNS, triggerChannel, true),
		//		NewDeployment(filterDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.FilterLabels(brokerName)),
		//			WithDeploymentServiceAccount(filterSA),
		//			WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), containerPorts(8080))),
		//		NewService(filterServiceName, testNS,
		//			WithServiceOwnerReferences(ownerReferences()),
		//			WithServiceLabels(resources.FilterLabels(brokerName)),
		//			WithServicePorts(servicePorts(filterContainerName, 8080))),
		//		NewDeployment(ingressDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.IngressLabels(brokerName)),
		//			WithDeploymentServiceAccount(ingressSA),
		//			WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080))),
		//		NewService(ingressServiceName, testNS,
		//			WithServiceOwnerReferences(ownerReferences()),
		//			WithServiceLabels(resources.IngressLabels(brokerName)),
		//			WithServicePorts(servicePorts(ingressContainerName, 8080))),
		//	},
		//	WithReactors: []clientgotesting.ReactionFunc{
		//		InduceFailure("create", "inmemorychannels"),
		//	},
		//	WantCreates: []runtime.Object{
		//		createChannelCRD(testNS, ingressChannel, false),
		//	},
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions,
		//			WithTriggerChannelReady(),
		//			WithBrokerTriggerChannel(createTriggerChannelCRDRef()),
		//			WithFilterDeploymentAvailable(),
		//			WithIngressDeploymentAvailable(),
		//			WithBrokerAddress(fmt.Sprintf("%s.%s.svc.%s", ingressServiceName, testNS, utils.GetClusterDomainName())),
		//			WithIngressChannelFailed("ChannelFailure", "inducing failure for create inmemorychannels")),
		//	}},
		//	WantEvents: []string{
		//		Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for create inmemorychannels"),
		//	},
		//	WantErr: true,
		//},
		//{
		//	Name: "Subscription.Create error",
		//	Key:  testKey,
		//	Objects: []runtime.Object{
		//		NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions),
		//		createChannelCRD(testNS, triggerChannel, true),
		//		createChannelCRD(testNS, ingressChannel, true),
		//		NewDeployment(filterDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.FilterLabels(brokerName)),
		//			WithDeploymentServiceAccount(filterSA),
		//			WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), containerPorts(8080))),
		//		NewService(filterServiceName, testNS,
		//			WithServiceOwnerReferences(ownerReferences()),
		//			WithServiceLabels(resources.FilterLabels(brokerName)),
		//			WithServicePorts(servicePorts(filterContainerName, 8080))),
		//		NewDeployment(ingressDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.IngressLabels(brokerName)),
		//			WithDeploymentServiceAccount(ingressSA),
		//			WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080))),
		//		NewService(ingressServiceName, testNS,
		//			WithServiceOwnerReferences(ownerReferences()),
		//			WithServiceLabels(resources.IngressLabels(brokerName)),
		//			WithServicePorts(servicePorts(ingressContainerName, 8080))),
		//	},
		//	WantCreates: []runtime.Object{
		//		NewSubscription("", testNS,
		//			WithSubscriptionGenerateName(ingressSubscriptionGenerateName),
		//			WithSubscriptionOwnerReferences(ownerReferences()),
		//			WithSubscriptionLabels(ingressSubscriptionLabels(brokerName)),
		//			WithSubscriptionChannel(imcGVK, ingressChannelName),
		//			WithSubscriptionSubscriberRef(serviceGVK, ingressServiceName)),
		//	},
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions,
		//			WithTriggerChannelReady(),
		//			WithBrokerTriggerChannel(createTriggerChannelCRDRef()),
		//			WithFilterDeploymentAvailable(),
		//			WithIngressDeploymentAvailable(),
		//			WithBrokerAddress(fmt.Sprintf("%s.%s.svc.%s", ingressServiceName, testNS, utils.GetClusterDomainName())),
		//			WithBrokerIngressChannelReady(),
		//			WithBrokerIngressChannel(createIngressChannelCRDRef()),
		//			WithBrokerIngressSubscriptionFailed("SubscriptionFailure", "inducing failure for create subscriptions"),
		//		),
		//	}},
		//	WantEvents: []string{
		//		Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for create subscriptions"),
		//	},
		//	WithReactors: []clientgotesting.ReactionFunc{
		//		InduceFailure("create", "subscriptions"),
		//	},
		//	WantErr: true,
		//},
		//{
		//	Name: "Subscription.Delete error",
		//	Key:  testKey,
		//	Objects: []runtime.Object{
		//		NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions),
		//		createChannelCRD(testNS, triggerChannel, true),
		//		createChannelCRD(testNS, ingressChannel, true),
		//		NewDeployment(filterDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.FilterLabels(brokerName)),
		//			WithDeploymentServiceAccount(filterSA),
		//			WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), containerPorts(8080))),
		//		NewService(filterServiceName, testNS,
		//			WithServiceOwnerReferences(ownerReferences()),
		//			WithServiceLabels(resources.FilterLabels(brokerName)),
		//			WithServicePorts(servicePorts(filterContainerName, 8080))),
		//		NewDeployment(ingressDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.IngressLabels(brokerName)),
		//			WithDeploymentServiceAccount(ingressSA),
		//			WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080))),
		//		NewService(ingressServiceName, testNS,
		//			WithServiceOwnerReferences(ownerReferences()),
		//			WithServiceLabels(resources.IngressLabels(brokerName)),
		//			WithServicePorts(servicePorts(ingressContainerName, 8080))),
		//		NewSubscription("subs", testNS,
		//			WithSubscriptionGenerateName(ingressSubscriptionGenerateName),
		//			WithSubscriptionOwnerReferences(ownerReferences()),
		//			WithSubscriptionLabels(ingressSubscriptionLabels(brokerName)),
		//			WithSubscriptionChannel(channelGVK, ingressChannelName),
		//			WithSubscriptionSubscriberRef(serviceGVK, "")),
		//	},
		//	WantDeletes: []clientgotesting.DeleteActionImpl{{
		//		Name: "subs",
		//	}},
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions,
		//			WithTriggerChannelReady(),
		//			WithBrokerTriggerChannel(createTriggerChannelCRDRef()),
		//			WithFilterDeploymentAvailable(),
		//			WithIngressDeploymentAvailable(),
		//			WithBrokerAddress(fmt.Sprintf("%s.%s.svc.%s", ingressServiceName, testNS, utils.GetClusterDomainName())),
		//			WithBrokerIngressChannelReady(),
		//			WithBrokerIngressChannel(createIngressChannelCRDRef()),
		//			WithBrokerIngressSubscriptionFailed("SubscriptionFailure", "inducing failure for delete subscriptions"),
		//		),
		//	}},
		//	WantEvents: []string{
		//		Eventf(corev1.EventTypeWarning, ingressSubscriptionDeleteFailed, "%v", "Delete Broker Ingress' subscription failed: inducing failure for delete subscriptions"),
		//		Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for delete subscriptions"),
		//	},
		//	WithReactors: []clientgotesting.ReactionFunc{
		//		InduceFailure("delete", "subscriptions"),
		//	},
		//	WantErr: true,
		//},
		//{
		//	Name: "Subscription.Create error when recreating",
		//	Key:  testKey,
		//	Objects: []runtime.Object{
		//		NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions),
		//		createChannelCRD(testNS, triggerChannel, true),
		//		createChannelCRD(testNS, ingressChannel, true),
		//		NewDeployment(filterDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.FilterLabels(brokerName)),
		//			WithDeploymentServiceAccount(filterSA),
		//			WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), containerPorts(8080))),
		//		NewService(filterServiceName, testNS,
		//			WithServiceOwnerReferences(ownerReferences()),
		//			WithServiceLabels(resources.FilterLabels(brokerName)),
		//			WithServicePorts(servicePorts(filterContainerName, 8080))),
		//		NewDeployment(ingressDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.IngressLabels(brokerName)),
		//			WithDeploymentServiceAccount(ingressSA),
		//			WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080))),
		//		NewService(ingressServiceName, testNS,
		//			WithServiceOwnerReferences(ownerReferences()),
		//			WithServiceLabels(resources.IngressLabels(brokerName)),
		//			WithServicePorts(servicePorts(ingressContainerName, 8080))),
		//		NewSubscription("subs", testNS,
		//			WithSubscriptionGenerateName(ingressSubscriptionGenerateName),
		//			WithSubscriptionOwnerReferences(ownerReferences()),
		//			WithSubscriptionLabels(ingressSubscriptionLabels(brokerName)),
		//			WithSubscriptionChannel(channelGVK, ingressChannelName),
		//			WithSubscriptionSubscriberRef(serviceGVK, "")),
		//	},
		//	WantDeletes: []clientgotesting.DeleteActionImpl{{
		//		Name: "subs",
		//	}},
		//	WantCreates: []runtime.Object{
		//		NewSubscription("", testNS,
		//			WithSubscriptionGenerateName(ingressSubscriptionGenerateName),
		//			WithSubscriptionOwnerReferences(ownerReferences()),
		//			WithSubscriptionLabels(ingressSubscriptionLabels(brokerName)),
		//			WithSubscriptionChannel(imcGVK, ingressChannelName),
		//			WithSubscriptionSubscriberRef(serviceGVK, ingressServiceName)),
		//	},
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions,
		//			WithTriggerChannelReady(),
		//			WithBrokerTriggerChannel(createTriggerChannelCRDRef()),
		//			WithFilterDeploymentAvailable(),
		//			WithIngressDeploymentAvailable(),
		//			WithBrokerAddress(fmt.Sprintf("%s.%s.svc.%s", ingressServiceName, testNS, utils.GetClusterDomainName())),
		//			WithBrokerIngressChannelReady(),
		//			WithBrokerIngressChannel(createIngressChannelCRDRef()),
		//			WithBrokerIngressSubscriptionFailed("SubscriptionFailure", "inducing failure for create subscriptions"),
		//		),
		//	}},
		//	WantEvents: []string{
		//		Eventf(corev1.EventTypeWarning, ingressSubscriptionCreateFailed, "%v", "Create Broker Ingress' subscription failed: inducing failure for create subscriptions"),
		//		Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for create subscriptions"),
		//	},
		//	WithReactors: []clientgotesting.ReactionFunc{
		//		InduceFailure("create", "subscriptions"),
		//	},
		//	WantErr: true,
		//},
		//{
		//	Name: "Successful Reconciliation",
		//	Key:  testKey,
		//	Objects: []runtime.Object{
		//		NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithInitBrokerConditions),
		//		createChannelCRD(testNS, triggerChannel, true),
		//		createChannelCRD(testNS, ingressChannel, true),
		//		NewDeployment(filterDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.FilterLabels(brokerName)),
		//			WithDeploymentServiceAccount(filterSA),
		//			WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), containerPorts(8080))),
		//		NewService(filterServiceName, testNS,
		//			WithServiceOwnerReferences(ownerReferences()),
		//			WithServiceLabels(resources.FilterLabels(brokerName)),
		//			WithServicePorts(servicePorts(filterContainerName, 8080))),
		//		NewDeployment(ingressDeploymentName, testNS,
		//			WithDeploymentOwnerReferences(ownerReferences()),
		//			WithDeploymentLabels(resources.IngressLabels(brokerName)),
		//			WithDeploymentServiceAccount(ingressSA),
		//			WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080))),
		//		NewService(ingressServiceName, testNS,
		//			WithServiceOwnerReferences(ownerReferences()),
		//			WithServiceLabels(resources.IngressLabels(brokerName)),
		//			WithServicePorts(servicePorts(ingressContainerName, 8080))),
		//		NewSubscription("", testNS,
		//			WithSubscriptionGenerateName(ingressSubscriptionGenerateName),
		//			WithSubscriptionOwnerReferences(ownerReferences()),
		//			WithSubscriptionLabels(ingressSubscriptionLabels(brokerName)),
		//			WithSubscriptionChannel(imcGVK, ingressChannelName),
		//			WithSubscriptionSubscriberRef(serviceGVK, ingressServiceName),
		//			WithSubscriptionReady),
		//	},
		//	WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
		//		Object: NewBroker(brokerName, testNS,
		//			WithBrokerChannelCRD(channelCRD()),
		//			WithBrokerReady,
		//			WithBrokerTriggerChannel(createTriggerChannelCRDRef()),
		//			WithBrokerIngressChannel(createIngressChannelCRDRef()),
		//			WithBrokerAddress(fmt.Sprintf("%s.%s.svc.%s", ingressServiceName, testNS, utils.GetClusterDomainName())),
		//		),
		//	}},
		//	WantEvents: []string{
		//		Eventf(corev1.EventTypeNormal, brokerReadinessChanged, "Broker %q became ready", brokerName),
		//	},
		// },
	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		return &Reconciler{
			Base:            reconciler.NewBase(ctx, controllerAgentName, cmw),
			channelLister:   listers.GetMessagingChannelLister(),
			resourceTracker: fakeResourceTracker{},
		}
	},
		false,
	))
}

func ownerReferences() []metav1.OwnerReference {
	return []metav1.OwnerReference{{
		APIVersion:         v1alpha1.SchemeGroupVersion.String(),
		Kind:               "Channel",
		Name:               channelName,
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
	}}
}

func channelCRD() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: "messaging.knative.dev/v1alpha1",
		Kind:       "InMemoryChannel",
	}
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

func servicePorts(containerName string, httpInternal int) []corev1.ServicePort {
	svcPorts := []corev1.ServicePort{
		{
			Name:       "http",
			Port:       80,
			TargetPort: intstr.FromInt(httpInternal),
		}, {
			Name: "metrics",
			Port: 9090,
		},
	}
	return svcPorts
}

func createChannelCRD(namespace, name string, ready bool) *unstructured.Unstructured {
	unstructured := &unstructured.Unstructured{}
	if ready {
		unstructured.Object = map[string]interface{}{
			"apiVersion": "messaging.knative.dev/v1alpha1",
			"kind":       "InMemoryChannel",
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"namespace":         namespace,
				"name":              name,
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "messaging.knative.dev/v1alpha1",
						"blockOwnerDeletion": true,
						"controller":         true,
						"kind":               "Channel",
						"name":               name,
						"uid":                "",
					},
				},
			},
			"status": map[string]interface{}{
				"address": map[string]interface{}{
					"hostname": backingChannelHostname,
					"url":      fmt.Sprintf("http://%s", backingChannelHostname),
				},
			},
		}
	} else {
		unstructured.Object = map[string]interface{}{
			"apiVersion": "messaging.knative.dev/v1alpha1",
			"kind":       "InMemoryChannel",
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"namespace":         namespace,
				"name":              name,
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "messaging.knative.dev/v1alpha1",
						"blockOwnerDeletion": true,
						"controller":         true,
						"kind":               "Channel",
						"name":               name,
						"uid":                "",
					},
				},
			},
		}
	}
	return unstructured
}

//func createTriggerChannelCRDRef() *corev1.ObjectReference {
//	return &corev1.ObjectReference{
//		APIVersion: "messaging.knative.dev/v1alpha1",
//		Kind:       "InMemoryChannel",
//		Namespace:  testNS,
//		Name:       fmt.Sprintf("%s-kn-trigger", brokerName),
//	}
//}
//
//func createIngressChannelCRDRef() *corev1.ObjectReference {
//	return &corev1.ObjectReference{
//		APIVersion: "messaging.knative.dev/v1alpha1",
//		Kind:       "InMemoryChannel",
//		Namespace:  testNS,
//		Name:       fmt.Sprintf("%s-kn-ingress", brokerName),
//	}
//}
