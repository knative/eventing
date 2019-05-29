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
	"fmt"
	"testing"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/eventing/pkg/reconciler/broker/resources"
	. "github.com/knative/eventing/pkg/reconciler/testing"
	"github.com/knative/eventing/pkg/utils"
	"github.com/knative/pkg/controller"
	logtesting "github.com/knative/pkg/logging/testing"
	. "github.com/knative/pkg/reconciler/testing"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
)

const (
	testNS     = "test-namespace"
	brokerName = "test-broker"

	filterImage  = "filter-image"
	filterSA     = "filter-SA"
	ingressImage = "ingress-image"
	ingressSA    = "ingress-SA"

	filterContainerName  = "filter"
	ingressContainerName = "ingress"
)

var (
	trueVal = true

	testKey                 = fmt.Sprintf("%s/%s", testNS, brokerName)
	channelGenerateName     = fmt.Sprintf("%s-broker-", brokerName)
	subscriptionChannelName = fmt.Sprintf("%s-broker", brokerName)

	triggerChannelHostname = fmt.Sprintf("foo.bar.svc.%s", utils.GetClusterDomainName())
	ingressChannelHostname = fmt.Sprintf("baz.qux.svc.%s", utils.GetClusterDomainName())

	filterDeploymentName  = fmt.Sprintf("%s-broker-filter", brokerName)
	filterServiceName     = fmt.Sprintf("%s-broker-filter", brokerName)
	ingressDeploymentName = fmt.Sprintf("%s-broker-ingress", brokerName)
	ingressServiceName    = fmt.Sprintf("%s-broker", brokerName)

	ingressSubscriptionGenerateName = fmt.Sprintf("internal-ingress-%s-", brokerName)

	channelGVK = metav1.GroupVersionKind{
		Group:   "eventing.knative.dev",
		Version: "v1alpha1",
		Kind:    "Channel",
	}

	serviceGVK = metav1.GroupVersionKind{
		Version: "v1",
		Kind:    "Service",
	}

	provisionerGVK = metav1.GroupVersionKind{
		Group:   "eventing.knative.dev",
		Version: "v1alpha1",
		Kind:    "ClusterChannelProvisioner",
	}
)

func init() {
	// Add types to scheme
	_ = v1alpha1.AddToScheme(scheme.Scheme)
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
			Name: "Broker not found",
			Key:  testKey,
		},
		{
			Name: "Broker is being deleted",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions,
					WithBrokerDeletionTimestamp),
			},
		},
		{
			Name: "Trigger Channel.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions),
			},
			WantCreates: []runtime.Object{
				NewChannel("", testNS,
					WithChannelGenerateName(channelGenerateName),
					WithChannelLabels(TriggerChannelLabels(brokerName)),
					WithChannelOwnerReferences(ownerReferences()),
					WithChannelProvisioner(provisionerGVK, "my-provisioner")),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithInitBrokerConditions,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithTriggerChannelFailed("ChannelFailure", "inducing failure for create channels")),
			}},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "channels"),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for create channels"),
			},
			WantErr: true,
		},
		{
			Name: "Trigger Channel is not yet Addressable",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions),
				NewChannel("", testNS,
					WithInitChannelConditions,
					WithChannelGenerateName(channelGenerateName),
					WithChannelLabels(TriggerChannelLabels(brokerName)),
					WithChannelOwnerReferences(ownerReferences()),
					WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					WithChannelAddress("")),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions,
					WithTriggerChannelFailed("NoAddress", "Channel does not have an address.")),
			}},
		},
		{
			Name: "Filter Deployment.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions),
				NewChannel("", testNS,
					WithChannelGenerateName(channelGenerateName),
					WithChannelLabels(TriggerChannelLabels(brokerName)),
					WithChannelOwnerReferences(ownerReferences()),
					WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					WithChannelReady,
					WithChannelAddress(triggerChannelHostname)),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "deployments"),
			},
			WantCreates: []runtime.Object{
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), nil)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions,
					WithTriggerChannelReady(),
					WithFilterFailed("DeploymentFailure", "inducing failure for create deployments")),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for create deployments"),
			},
			WantErr: true,
		},
		{
			Name: "Filter Deployment.Update error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions),
				NewChannel("", testNS,
					WithChannelGenerateName(channelGenerateName),
					WithChannelLabels(TriggerChannelLabels(brokerName)),
					WithChannelOwnerReferences(ownerReferences()),
					WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					WithChannelReady,
					WithChannelAddress(triggerChannelHostname)),
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, "some-other-image", envVars(filterContainerName), nil)),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "deployments"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions,
					WithTriggerChannelReady(),
					WithFilterFailed("DeploymentFailure", "inducing failure for update deployments")),
			}},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), nil)),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for update deployments"),
			},
			WantErr: true,
		},
		{
			Name: "Filter Service.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions),
				NewChannel("", testNS,
					WithChannelGenerateName(channelGenerateName),
					WithChannelLabels(TriggerChannelLabels(brokerName)),
					WithChannelOwnerReferences(ownerReferences()),
					WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					WithChannelReady,
					WithChannelAddress(triggerChannelHostname)),
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), nil)),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "services"),
			},
			WantCreates: []runtime.Object{
				NewService(filterServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.FilterLabels(brokerName)),
					WithServicePorts(servicePorts(filterContainerName, 8080))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions,
					WithTriggerChannelReady(),
					WithFilterFailed("ServiceFailure", "inducing failure for create services")),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for create services"),
			},
			WantErr: true,
		},
		{
			Name: "Filter Service.Update error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions),
				NewChannel("", testNS,
					WithChannelGenerateName(channelGenerateName),
					WithChannelLabels(TriggerChannelLabels(brokerName)),
					WithChannelOwnerReferences(ownerReferences()),
					WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					WithChannelReady,
					WithChannelAddress(triggerChannelHostname)),
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), nil)),
				NewService(filterServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.FilterLabels(brokerName)),
					WithServicePorts(servicePorts(filterContainerName, 9090))),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "services"),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewService(filterServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.FilterLabels(brokerName)),
					WithServicePorts(servicePorts(filterContainerName, 8080))),
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions,
					WithTriggerChannelReady(),
					WithFilterFailed("ServiceFailure", "inducing failure for update services")),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for update services"),
			},
			WantErr: true,
		},
		{
			Name: "Ingress Deployment.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions),
				NewChannel("", testNS,
					WithChannelGenerateName(channelGenerateName),
					WithChannelLabels(TriggerChannelLabels(brokerName)),
					WithChannelOwnerReferences(ownerReferences()),
					WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					WithChannelReady,
					WithChannelAddress(triggerChannelHostname)),
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), nil)),
				NewService(filterServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.FilterLabels(brokerName)),
					WithServicePorts(servicePorts(filterContainerName, 8080))),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "deployments"),
			},
			WantCreates: []runtime.Object{
				NewDeployment(ingressDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.IngressLabels(brokerName)),
					WithDeploymentServiceAccount(ingressSA),
					WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080)),
				),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions,
					WithTriggerChannelReady(),
					WithFilterDeploymentAvailable(),
					WithIngressFailed("DeploymentFailure", "inducing failure for create deployments")),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for create deployments"),
			},
			WantErr: true,
		},
		{
			Name: "Ingress Deployment.Update error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions),
				NewChannel("", testNS,
					WithChannelGenerateName(channelGenerateName),
					WithChannelLabels(TriggerChannelLabels(brokerName)),
					WithChannelOwnerReferences(ownerReferences()),
					WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					WithChannelReady,
					WithChannelAddress(triggerChannelHostname)),
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), nil)),
				NewService(filterServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.FilterLabels(brokerName)),
					WithServicePorts(servicePorts(filterContainerName, 8080))),
				NewDeployment(ingressDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.IngressLabels(brokerName)),
					WithDeploymentServiceAccount(ingressSA),
					WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(9090))),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "deployments"),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewDeployment(ingressDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.IngressLabels(brokerName)),
					WithDeploymentServiceAccount(ingressSA),
					WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080))),
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions,
					WithTriggerChannelReady(),
					WithFilterDeploymentAvailable(),
					WithIngressFailed("DeploymentFailure", "inducing failure for update deployments")),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for update deployments"),
			},
			WantErr: true,
		},
		{
			Name: "Ingress Service.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions),
				NewChannel("", testNS,
					WithChannelGenerateName(channelGenerateName),
					WithChannelLabels(TriggerChannelLabels(brokerName)),
					WithChannelOwnerReferences(ownerReferences()),
					WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					WithChannelReady,
					WithChannelAddress(triggerChannelHostname)),
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), nil)),
				NewService(filterServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.FilterLabels(brokerName)),
					WithServicePorts(servicePorts(filterContainerName, 8080))),
				NewDeployment(ingressDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.IngressLabels(brokerName)),
					WithDeploymentServiceAccount(ingressSA),
					WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080))),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "services"),
			},
			WantCreates: []runtime.Object{
				NewService(ingressServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.IngressLabels(brokerName)),
					WithServicePorts(servicePorts(ingressContainerName, 8080))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions,
					WithTriggerChannelReady(),
					WithFilterDeploymentAvailable(),
					WithIngressFailed("ServiceFailure", "inducing failure for create services")),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for create services"),
			},
			WantErr: true,
		},
		{
			Name: "Ingress Service.Update error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions),
				NewChannel("", testNS,
					WithChannelGenerateName(channelGenerateName),
					WithChannelLabels(TriggerChannelLabels(brokerName)),
					WithChannelOwnerReferences(ownerReferences()),
					WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					WithChannelReady,
					WithChannelAddress(triggerChannelHostname)),
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), nil)),
				NewService(filterServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.FilterLabels(brokerName)),
					WithServicePorts(servicePorts(filterContainerName, 8080))),
				NewDeployment(ingressDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.IngressLabels(brokerName)),
					WithDeploymentServiceAccount(ingressSA),
					WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080))),
				NewService(ingressServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.IngressLabels(brokerName)),
					WithServicePorts(servicePorts(ingressContainerName, 9090))),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "services"),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewService(ingressServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.IngressLabels(brokerName)),
					WithServicePorts(servicePorts(ingressContainerName, 8080))),
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions,
					WithTriggerChannelReady(),
					WithFilterDeploymentAvailable(),
					WithIngressFailed("ServiceFailure", "inducing failure for update services")),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for update services"),
			},
			WantErr: true,
		},
		{
			Name: "Ingress Channel.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions),
				NewChannel("", testNS,
					WithChannelGenerateName(channelGenerateName),
					WithChannelLabels(TriggerChannelLabels(brokerName)),
					WithChannelOwnerReferences(ownerReferences()),
					WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					WithChannelReady,
					WithChannelAddress(triggerChannelHostname)),
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), nil)),
				NewService(filterServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.FilterLabels(brokerName)),
					WithServicePorts(servicePorts(filterContainerName, 8080))),
				NewDeployment(ingressDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.IngressLabels(brokerName)),
					WithDeploymentServiceAccount(ingressSA),
					WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080))),
				NewService(ingressServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.IngressLabels(brokerName)),
					WithServicePorts(servicePorts(ingressContainerName, 8080))),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "channels"),
			},
			WantCreates: []runtime.Object{
				NewChannel("", testNS,
					WithChannelGenerateName(channelGenerateName),
					WithChannelLabels(IngressChannelLabels(brokerName)),
					WithChannelOwnerReferences(ownerReferences()),
					WithChannelProvisioner(provisionerGVK, "my-provisioner")),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions,
					WithTriggerChannelReady(),
					WithFilterDeploymentAvailable(),
					WithIngressDeploymentAvailable(),
					WithBrokerAddress(fmt.Sprintf("%s.%s.svc.%s", ingressServiceName, testNS, utils.GetClusterDomainName())),
					WithIngressChannelFailed("ChannelFailure", "inducing failure for create channels")),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for create channels"),
			},
			WantErr: true,
		},
		{
			Name: "Subscription.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions),
				// Use the channel name to avoid conflicting with the ingress one.
				NewChannel("filter-channel", testNS,
					WithChannelGenerateName(channelGenerateName),
					WithChannelLabels(TriggerChannelLabels(brokerName)),
					WithChannelOwnerReferences(ownerReferences()),
					WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					WithChannelReady,
					WithChannelAddress(triggerChannelHostname)),
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), nil)),
				NewService(filterServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.FilterLabels(brokerName)),
					WithServicePorts(servicePorts(filterContainerName, 8080))),
				NewDeployment(ingressDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.IngressLabels(brokerName)),
					WithDeploymentServiceAccount(ingressSA),
					WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080))),
				NewService(ingressServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.IngressLabels(brokerName)),
					WithServicePorts(servicePorts(ingressContainerName, 8080))),
				// Use the channel name to avoid conflicting with the filter one.
				NewChannel("ingress-channel", testNS,
					WithChannelGenerateName(channelGenerateName),
					WithChannelLabels(IngressChannelLabels(brokerName)),
					WithChannelOwnerReferences(ownerReferences()),
					WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					WithChannelReady,
					WithChannelAddress(ingressChannelHostname)),
			},
			WantCreates: []runtime.Object{
				NewSubscription("", testNS,
					WithSubscriptionGenerateName(ingressSubscriptionGenerateName),
					WithSubscriptionOwnerReferences(ownerReferences()),
					WithSubscriptionLabels(ingressSubscriptionLabels(brokerName)),
					WithSubscriptionChannel(channelGVK, "ingress-channel"),
					WithSubscriptionSubscriberRef(serviceGVK, ingressServiceName)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions,
					WithTriggerChannelReady(),
					WithFilterDeploymentAvailable(),
					WithIngressDeploymentAvailable(),
					WithBrokerAddress(fmt.Sprintf("%s.%s.svc.%s", ingressServiceName, testNS, utils.GetClusterDomainName())),
					WithBrokerIngressChannelReady(),
					WithBrokerIngressSubscriptionFailed("SubscriptionFailure", "inducing failure for create subscriptions"),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for create subscriptions"),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "subscriptions"),
			},
			WantErr: true,
		},
		{
			Name: "Subscription.Delete error",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions),
				// Use the channel name to avoid conflicting with the ingress one.
				NewChannel("filter-channel", testNS,
					WithChannelGenerateName(channelGenerateName),
					WithChannelLabels(TriggerChannelLabels(brokerName)),
					WithChannelOwnerReferences(ownerReferences()),
					WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					WithChannelReady,
					WithChannelAddress(triggerChannelHostname)),
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), nil)),
				NewService(filterServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.FilterLabels(brokerName)),
					WithServicePorts(servicePorts(filterContainerName, 8080))),
				NewDeployment(ingressDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.IngressLabels(brokerName)),
					WithDeploymentServiceAccount(ingressSA),
					WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080))),
				NewService(ingressServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.IngressLabels(brokerName)),
					WithServicePorts(servicePorts(ingressContainerName, 8080))),
				// Use the channel name to avoid conflicting with the filter one.
				NewChannel("ingress-channel", testNS,
					WithChannelGenerateName(channelGenerateName),
					WithChannelLabels(IngressChannelLabels(brokerName)),
					WithChannelOwnerReferences(ownerReferences()),
					WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					WithChannelReady,
					WithChannelAddress(ingressChannelHostname)),
				NewSubscription("subs", testNS,
					WithSubscriptionGenerateName(ingressSubscriptionGenerateName),
					WithSubscriptionOwnerReferences(ownerReferences()),
					WithSubscriptionLabels(ingressSubscriptionLabels(brokerName)),
					WithSubscriptionChannel(channelGVK, "ingress-channel"),
					WithSubscriptionSubscriberRef(serviceGVK, "")),
			},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				Name: "subs",
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions,
					WithTriggerChannelReady(),
					WithFilterDeploymentAvailable(),
					WithIngressDeploymentAvailable(),
					WithBrokerAddress(fmt.Sprintf("%s.%s.svc.%s", ingressServiceName, testNS, utils.GetClusterDomainName())),
					WithBrokerIngressChannelReady(),
					WithBrokerIngressSubscriptionFailed("SubscriptionFailure", "inducing failure for delete subscriptions"),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, ingressSubscriptionDeleteFailed, "%v", "Delete Broker Ingress' subscription failed: inducing failure for delete subscriptions"),
				Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for delete subscriptions"),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("delete", "subscriptions"),
			},
			WantErr: true,
		},
		{
			Name: "Subscription.Create error when recreating",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions),
				// Use the channel name to avoid conflicting with the ingress one.
				NewChannel("filter-channel", testNS,
					WithChannelGenerateName(channelGenerateName),
					WithChannelLabels(TriggerChannelLabels(brokerName)),
					WithChannelOwnerReferences(ownerReferences()),
					WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					WithChannelReady,
					WithChannelAddress(triggerChannelHostname)),
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), nil)),
				NewService(filterServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.FilterLabels(brokerName)),
					WithServicePorts(servicePorts(filterContainerName, 8080))),
				NewDeployment(ingressDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.IngressLabels(brokerName)),
					WithDeploymentServiceAccount(ingressSA),
					WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080))),
				NewService(ingressServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.IngressLabels(brokerName)),
					WithServicePorts(servicePorts(ingressContainerName, 8080))),
				// Use the channel name to avoid conflicting with the filter one.
				NewChannel("ingress-channel", testNS,
					WithChannelGenerateName(channelGenerateName),
					WithChannelLabels(IngressChannelLabels(brokerName)),
					WithChannelOwnerReferences(ownerReferences()),
					WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					WithChannelReady,
					WithChannelAddress(ingressChannelHostname)),
				NewSubscription("subs", testNS,
					WithSubscriptionGenerateName(ingressSubscriptionGenerateName),
					WithSubscriptionOwnerReferences(ownerReferences()),
					WithSubscriptionLabels(ingressSubscriptionLabels(brokerName)),
					WithSubscriptionChannel(channelGVK, "ingress-channel"),
					WithSubscriptionSubscriberRef(serviceGVK, "")),
			},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				Name: "subs",
			}},
			WantCreates: []runtime.Object{
				NewSubscription("", testNS,
					WithSubscriptionGenerateName(ingressSubscriptionGenerateName),
					WithSubscriptionOwnerReferences(ownerReferences()),
					WithSubscriptionLabels(ingressSubscriptionLabels(brokerName)),
					WithSubscriptionChannel(channelGVK, "ingress-channel"),
					WithSubscriptionSubscriberRef(serviceGVK, ingressServiceName)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions,
					WithTriggerChannelReady(),
					WithFilterDeploymentAvailable(),
					WithIngressDeploymentAvailable(),
					WithBrokerAddress(fmt.Sprintf("%s.%s.svc.%s", ingressServiceName, testNS, utils.GetClusterDomainName())),
					WithBrokerIngressChannelReady(),
					WithBrokerIngressSubscriptionFailed("SubscriptionFailure", "inducing failure for create subscriptions"),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, ingressSubscriptionCreateFailed, "%v", "Create Broker Ingress' subscription failed: inducing failure for create subscriptions"),
				Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for create subscriptions"),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "subscriptions"),
			},
			WantErr: true,
		},
		{
			Name: "Successful Reconciliation",
			Key:  testKey,
			Objects: []runtime.Object{
				NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithInitBrokerConditions),
				// Use the channel name to avoid conflicting with the ingress one.
				NewChannel("filter-channel", testNS,
					WithChannelGenerateName(channelGenerateName),
					WithChannelLabels(TriggerChannelLabels(brokerName)),
					WithChannelOwnerReferences(ownerReferences()),
					WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					WithChannelReady,
					WithChannelAddress(triggerChannelHostname)),
				NewDeployment(filterDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.FilterLabels(brokerName)),
					WithDeploymentServiceAccount(filterSA),
					WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), nil)),
				NewService(filterServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.FilterLabels(brokerName)),
					WithServicePorts(servicePorts(filterContainerName, 8080))),
				NewDeployment(ingressDeploymentName, testNS,
					WithDeploymentOwnerReferences(ownerReferences()),
					WithDeploymentLabels(resources.IngressLabels(brokerName)),
					WithDeploymentServiceAccount(ingressSA),
					WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080))),
				NewService(ingressServiceName, testNS,
					WithServiceOwnerReferences(ownerReferences()),
					WithServiceLabels(resources.IngressLabels(brokerName)),
					WithServicePorts(servicePorts(ingressContainerName, 8080))),
				// Use the channel name to avoid conflicting with the filter one.
				NewChannel("ingress-channel", testNS,
					WithChannelGenerateName(channelGenerateName),
					WithChannelLabels(IngressChannelLabels(brokerName)),
					WithChannelOwnerReferences(ownerReferences()),
					WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					WithChannelReady,
					WithChannelAddress(ingressChannelHostname)),
				NewSubscription("", testNS,
					WithSubscriptionGenerateName(ingressSubscriptionGenerateName),
					WithSubscriptionOwnerReferences(ownerReferences()),
					WithSubscriptionLabels(ingressSubscriptionLabels(brokerName)),
					WithSubscriptionChannel(channelGVK, "ingress-channel"),
					WithSubscriptionSubscriberRef(serviceGVK, ingressServiceName),
					WithSubscriptionReady),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewBroker(brokerName, testNS,
					WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					WithBrokerReady,
					WithBrokerAddress(fmt.Sprintf("%s.%s.svc.%s", ingressServiceName, testNS, utils.GetClusterDomainName())),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, brokerReadinessChanged, "Broker %q became ready", brokerName),
			},
		},
	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(listers *Listers, opt reconciler.Options) controller.Reconciler {
		return &Reconciler{
			Base:                      reconciler.NewBase(opt, controllerAgentName),
			subscriptionLister:        listers.GetSubscriptionLister(),
			brokerLister:              listers.GetBrokerLister(),
			channelLister:             listers.GetChannelLister(),
			serviceLister:             listers.GetK8sServiceLister(),
			deploymentLister:          listers.GetDeploymentLister(),
			filterImage:               filterImage,
			filterServiceAccountName:  filterSA,
			ingressImage:              ingressImage,
			ingressServiceAccountName: ingressSA,
		}
	},
		false,
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

func channelProvisioner(name string) *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: "eventing.knative.dev/v1alpha1",
		Kind:       "ClusterChannelProvisioner",
		Name:       name,
	}
}

func envVars(containerName string) []corev1.EnvVar {
	switch containerName {
	case filterContainerName:
		return []corev1.EnvVar{
			{
				Name: "NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
		}
	case ingressContainerName:
		return []corev1.EnvVar{
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

func servicePorts(containerName string, httpInternal int) []corev1.ServicePort {
	svcPorts := []corev1.ServicePort{
		{
			Name:       "http",
			Port:       80,
			TargetPort: intstr.FromInt(httpInternal),
		},
	}
	// TODO remove this if once we add metrics to the filter container.
	if containerName == ingressContainerName {
		svcPorts = append(svcPorts, corev1.ServicePort{
			Name: "metrics",
			Port: 9090,
		})
	}
	return svcPorts
}
