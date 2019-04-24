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

	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/reconciler"
	testing2 "github.com/knative/eventing/pkg/reconciler/testing"
	"github.com/knative/eventing/pkg/reconciler/v1alpha1/broker/resources"
	"github.com/knative/eventing/pkg/utils"
	"github.com/knative/pkg/controller"
	logtesting "github.com/knative/pkg/logging/testing"
	. "github.com/knative/pkg/reconciler/testing"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		Group:   "apps",
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
				testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions,
					testing2.WithBrokerDeletionTimestamp),
			},
		},
		{
			Name: "Trigger Channel.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions),
			},
			WantCreates: []metav1.Object{
				testing2.NewChannel("", testNS,
					testing2.WithChannelGenerateName(channelGenerateName),
					testing2.WithChannelLabels(TriggerChannelLabels(brokerName)),
					testing2.WithChannelOwnerReferences(ownerReferences()),
					testing2.WithChannelProvisioner(provisionerGVK, "my-provisioner")),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewBroker(brokerName, testNS,
					testing2.WithInitBrokerConditions,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithTriggerChannelFailed("ChannelFailure", "inducing failure for create channels")),
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
				testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions),
				testing2.NewChannel("", testNS,
					testing2.WithInitChannelConditions,
					testing2.WithChannelGenerateName(channelGenerateName),
					testing2.WithChannelLabels(TriggerChannelLabels(brokerName)),
					testing2.WithChannelOwnerReferences(ownerReferences()),
					testing2.WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					testing2.WithChannelAddress("")),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions,
					testing2.WithTriggerChannelFailed("NoAddress", "Channel does not have an address.")),
			}},
		},
		{
			Name: "Filter Deployment.Create error",
			Key:  testKey,
			Objects: []runtime.Object{
				testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions),
				testing2.NewChannel("", testNS,
					testing2.WithChannelGenerateName(channelGenerateName),
					testing2.WithChannelLabels(TriggerChannelLabels(brokerName)),
					testing2.WithChannelOwnerReferences(ownerReferences()),
					testing2.WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					testing2.WithChannelReady,
					testing2.WithChannelAddress(triggerChannelHostname)),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "deployments"),
			},
			WantCreates: []metav1.Object{
				testing2.NewDeployment(filterDeploymentName, testNS,
					testing2.WithDeploymentOwnerReferences(ownerReferences()),
					testing2.WithDeploymentLabels(resources.FilterLabels(brokerName)),
					testing2.WithDeploymentAnnotations(annotations()),
					testing2.WithDeploymentServiceAccount(filterSA),
					testing2.WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), nil)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions,
					testing2.WithTriggerChannelReady(),
					testing2.WithFilterFailed("DeploymentFailure", "inducing failure for create deployments")),
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
				testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions),
				testing2.NewChannel("", testNS,
					testing2.WithChannelGenerateName(channelGenerateName),
					testing2.WithChannelLabels(TriggerChannelLabels(brokerName)),
					testing2.WithChannelOwnerReferences(ownerReferences()),
					testing2.WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					testing2.WithChannelReady,
					testing2.WithChannelAddress(triggerChannelHostname)),
				testing2.NewDeployment(filterDeploymentName, testNS,
					testing2.WithDeploymentOwnerReferences(ownerReferences()),
					testing2.WithDeploymentLabels(resources.FilterLabels(brokerName)),
					testing2.WithDeploymentAnnotations(annotations()),
					testing2.WithDeploymentServiceAccount(filterSA),
					testing2.WithDeploymentContainer(filterContainerName, "some-other-image", envVars(filterContainerName), nil)),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "deployments"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions,
					testing2.WithTriggerChannelReady(),
					testing2.WithFilterFailed("DeploymentFailure", "inducing failure for update deployments")),
			}},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewDeployment(filterDeploymentName, testNS,
					testing2.WithDeploymentOwnerReferences(ownerReferences()),
					testing2.WithDeploymentLabels(resources.FilterLabels(brokerName)),
					testing2.WithDeploymentAnnotations(annotations()),
					testing2.WithDeploymentServiceAccount(filterSA),
					testing2.WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), nil)),
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
				testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions),
				testing2.NewChannel("", testNS,
					testing2.WithChannelGenerateName(channelGenerateName),
					testing2.WithChannelLabels(TriggerChannelLabels(brokerName)),
					testing2.WithChannelOwnerReferences(ownerReferences()),
					testing2.WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					testing2.WithChannelReady,
					testing2.WithChannelAddress(triggerChannelHostname)),
				testing2.NewDeployment(filterDeploymentName, testNS,
					testing2.WithDeploymentOwnerReferences(ownerReferences()),
					testing2.WithDeploymentLabels(resources.FilterLabels(brokerName)),
					testing2.WithDeploymentAnnotations(annotations()),
					testing2.WithDeploymentServiceAccount(filterSA),
					testing2.WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), nil)),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "services"),
			},
			WantCreates: []metav1.Object{
				testing2.NewService(filterServiceName, testNS,
					testing2.WithServiceOwnerReferences(ownerReferences()),
					testing2.WithServiceLabels(resources.FilterLabels(brokerName)),
					testing2.WithServicePorts(servicePorts(filterContainerName, 8080))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions,
					testing2.WithTriggerChannelReady(),
					testing2.WithFilterFailed("ServiceFailure", "inducing failure for create services")),
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
				testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions),
				testing2.NewChannel("", testNS,
					testing2.WithChannelGenerateName(channelGenerateName),
					testing2.WithChannelLabels(TriggerChannelLabels(brokerName)),
					testing2.WithChannelOwnerReferences(ownerReferences()),
					testing2.WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					testing2.WithChannelReady,
					testing2.WithChannelAddress(triggerChannelHostname)),
				testing2.NewDeployment(filterDeploymentName, testNS,
					testing2.WithDeploymentOwnerReferences(ownerReferences()),
					testing2.WithDeploymentLabels(resources.FilterLabels(brokerName)),
					testing2.WithDeploymentAnnotations(annotations()),
					testing2.WithDeploymentServiceAccount(filterSA),
					testing2.WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), nil)),
				testing2.NewService(filterServiceName, testNS,
					testing2.WithServiceOwnerReferences(ownerReferences()),
					testing2.WithServiceLabels(resources.FilterLabels(brokerName)),
					testing2.WithServicePorts(servicePorts(filterContainerName, 9090))),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "services"),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewService(filterServiceName, testNS,
					testing2.WithServiceOwnerReferences(ownerReferences()),
					testing2.WithServiceLabels(resources.FilterLabels(brokerName)),
					testing2.WithServicePorts(servicePorts(filterContainerName, 8080))),
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions,
					testing2.WithTriggerChannelReady(),
					testing2.WithFilterFailed("ServiceFailure", "inducing failure for update services")),
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
				testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions),
				testing2.NewChannel("", testNS,
					testing2.WithChannelGenerateName(channelGenerateName),
					testing2.WithChannelLabels(TriggerChannelLabels(brokerName)),
					testing2.WithChannelOwnerReferences(ownerReferences()),
					testing2.WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					testing2.WithChannelReady,
					testing2.WithChannelAddress(triggerChannelHostname)),
				testing2.NewDeployment(filterDeploymentName, testNS,
					testing2.WithDeploymentOwnerReferences(ownerReferences()),
					testing2.WithDeploymentLabels(resources.FilterLabels(brokerName)),
					testing2.WithDeploymentAnnotations(annotations()),
					testing2.WithDeploymentServiceAccount(filterSA),
					testing2.WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), nil)),
				testing2.NewService(filterServiceName, testNS,
					testing2.WithServiceOwnerReferences(ownerReferences()),
					testing2.WithServiceLabels(resources.FilterLabels(brokerName)),
					testing2.WithServicePorts(servicePorts(filterContainerName, 8080))),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "deployments"),
			},
			WantCreates: []metav1.Object{
				testing2.NewDeployment(ingressDeploymentName, testNS,
					testing2.WithDeploymentOwnerReferences(ownerReferences()),
					testing2.WithDeploymentLabels(resources.IngressLabels(brokerName)),
					testing2.WithDeploymentAnnotations(annotations()),
					testing2.WithDeploymentServiceAccount(ingressSA),
					testing2.WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080)),
				),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions,
					testing2.WithTriggerChannelReady(),
					testing2.WithFilterDeploymentAvailable(),
					testing2.WithIngressFailed("DeploymentFailure", "inducing failure for create deployments")),
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
				testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions),
				testing2.NewChannel("", testNS,
					testing2.WithChannelGenerateName(channelGenerateName),
					testing2.WithChannelLabels(TriggerChannelLabels(brokerName)),
					testing2.WithChannelOwnerReferences(ownerReferences()),
					testing2.WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					testing2.WithChannelReady,
					testing2.WithChannelAddress(triggerChannelHostname)),
				testing2.NewDeployment(filterDeploymentName, testNS,
					testing2.WithDeploymentOwnerReferences(ownerReferences()),
					testing2.WithDeploymentLabels(resources.FilterLabels(brokerName)),
					testing2.WithDeploymentAnnotations(annotations()),
					testing2.WithDeploymentServiceAccount(filterSA),
					testing2.WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), nil)),
				testing2.NewService(filterServiceName, testNS,
					testing2.WithServiceOwnerReferences(ownerReferences()),
					testing2.WithServiceLabels(resources.FilterLabels(brokerName)),
					testing2.WithServicePorts(servicePorts(filterContainerName, 8080))),
				testing2.NewDeployment(ingressDeploymentName, testNS,
					testing2.WithDeploymentOwnerReferences(ownerReferences()),
					testing2.WithDeploymentLabels(resources.IngressLabels(brokerName)),
					testing2.WithDeploymentAnnotations(annotations()),
					testing2.WithDeploymentServiceAccount(ingressSA),
					testing2.WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(9090))),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "deployments"),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewDeployment(ingressDeploymentName, testNS,
					testing2.WithDeploymentOwnerReferences(ownerReferences()),
					testing2.WithDeploymentLabels(resources.IngressLabels(brokerName)),
					testing2.WithDeploymentAnnotations(annotations()),
					testing2.WithDeploymentServiceAccount(ingressSA),
					testing2.WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080))),
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions,
					testing2.WithTriggerChannelReady(),
					testing2.WithFilterDeploymentAvailable(),
					testing2.WithIngressFailed("DeploymentFailure", "inducing failure for update deployments")),
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
				testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions),
				testing2.NewChannel("", testNS,
					testing2.WithChannelGenerateName(channelGenerateName),
					testing2.WithChannelLabels(TriggerChannelLabels(brokerName)),
					testing2.WithChannelOwnerReferences(ownerReferences()),
					testing2.WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					testing2.WithChannelReady,
					testing2.WithChannelAddress(triggerChannelHostname)),
				testing2.NewDeployment(filterDeploymentName, testNS,
					testing2.WithDeploymentOwnerReferences(ownerReferences()),
					testing2.WithDeploymentLabels(resources.FilterLabels(brokerName)),
					testing2.WithDeploymentAnnotations(annotations()),
					testing2.WithDeploymentServiceAccount(filterSA),
					testing2.WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), nil)),
				testing2.NewService(filterServiceName, testNS,
					testing2.WithServiceOwnerReferences(ownerReferences()),
					testing2.WithServiceLabels(resources.FilterLabels(brokerName)),
					testing2.WithServicePorts(servicePorts(filterContainerName, 8080))),
				testing2.NewDeployment(ingressDeploymentName, testNS,
					testing2.WithDeploymentOwnerReferences(ownerReferences()),
					testing2.WithDeploymentLabels(resources.IngressLabels(brokerName)),
					testing2.WithDeploymentAnnotations(annotations()),
					testing2.WithDeploymentServiceAccount(ingressSA),
					testing2.WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080))),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "services"),
			},
			WantCreates: []metav1.Object{
				testing2.NewService(ingressServiceName, testNS,
					testing2.WithServiceOwnerReferences(ownerReferences()),
					testing2.WithServiceLabels(resources.IngressLabels(brokerName)),
					testing2.WithServicePorts(servicePorts(ingressContainerName, 8080))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions,
					testing2.WithTriggerChannelReady(),
					testing2.WithFilterDeploymentAvailable(),
					testing2.WithIngressFailed("ServiceFailure", "inducing failure for create services")),
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
				testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions),
				testing2.NewChannel("", testNS,
					testing2.WithChannelGenerateName(channelGenerateName),
					testing2.WithChannelLabels(TriggerChannelLabels(brokerName)),
					testing2.WithChannelOwnerReferences(ownerReferences()),
					testing2.WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					testing2.WithChannelReady,
					testing2.WithChannelAddress(triggerChannelHostname)),
				testing2.NewDeployment(filterDeploymentName, testNS,
					testing2.WithDeploymentOwnerReferences(ownerReferences()),
					testing2.WithDeploymentLabels(resources.FilterLabels(brokerName)),
					testing2.WithDeploymentAnnotations(annotations()),
					testing2.WithDeploymentServiceAccount(filterSA),
					testing2.WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), nil)),
				testing2.NewService(filterServiceName, testNS,
					testing2.WithServiceOwnerReferences(ownerReferences()),
					testing2.WithServiceLabels(resources.FilterLabels(brokerName)),
					testing2.WithServicePorts(servicePorts(filterContainerName, 8080))),
				testing2.NewDeployment(ingressDeploymentName, testNS,
					testing2.WithDeploymentOwnerReferences(ownerReferences()),
					testing2.WithDeploymentLabels(resources.IngressLabels(brokerName)),
					testing2.WithDeploymentAnnotations(annotations()),
					testing2.WithDeploymentServiceAccount(ingressSA),
					testing2.WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080))),
				testing2.NewService(ingressServiceName, testNS,
					testing2.WithServiceOwnerReferences(ownerReferences()),
					testing2.WithServiceLabels(resources.IngressLabels(brokerName)),
					testing2.WithServicePorts(servicePorts(ingressContainerName, 9090))),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "services"),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewService(ingressServiceName, testNS,
					testing2.WithServiceOwnerReferences(ownerReferences()),
					testing2.WithServiceLabels(resources.IngressLabels(brokerName)),
					testing2.WithServicePorts(servicePorts(ingressContainerName, 8080))),
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions,
					testing2.WithTriggerChannelReady(),
					testing2.WithFilterDeploymentAvailable(),
					testing2.WithIngressFailed("ServiceFailure", "inducing failure for update services")),
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
				testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions),
				testing2.NewChannel("", testNS,
					testing2.WithChannelGenerateName(channelGenerateName),
					testing2.WithChannelLabels(TriggerChannelLabels(brokerName)),
					testing2.WithChannelOwnerReferences(ownerReferences()),
					testing2.WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					testing2.WithChannelReady,
					testing2.WithChannelAddress(triggerChannelHostname)),
				testing2.NewDeployment(filterDeploymentName, testNS,
					testing2.WithDeploymentOwnerReferences(ownerReferences()),
					testing2.WithDeploymentLabels(resources.FilterLabels(brokerName)),
					testing2.WithDeploymentAnnotations(annotations()),
					testing2.WithDeploymentServiceAccount(filterSA),
					testing2.WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), nil)),
				testing2.NewService(filterServiceName, testNS,
					testing2.WithServiceOwnerReferences(ownerReferences()),
					testing2.WithServiceLabels(resources.FilterLabels(brokerName)),
					testing2.WithServicePorts(servicePorts(filterContainerName, 8080))),
				testing2.NewDeployment(ingressDeploymentName, testNS,
					testing2.WithDeploymentOwnerReferences(ownerReferences()),
					testing2.WithDeploymentLabels(resources.IngressLabels(brokerName)),
					testing2.WithDeploymentAnnotations(annotations()),
					testing2.WithDeploymentServiceAccount(ingressSA),
					testing2.WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080))),
				testing2.NewService(ingressServiceName, testNS,
					testing2.WithServiceOwnerReferences(ownerReferences()),
					testing2.WithServiceLabels(resources.IngressLabels(brokerName)),
					testing2.WithServicePorts(servicePorts(ingressContainerName, 8080))),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "channels"),
			},
			WantCreates: []metav1.Object{
				testing2.NewChannel("", testNS,
					testing2.WithChannelGenerateName(channelGenerateName),
					testing2.WithChannelLabels(IngressChannelLabels(brokerName)),
					testing2.WithChannelOwnerReferences(ownerReferences()),
					testing2.WithChannelProvisioner(provisionerGVK, "my-provisioner")),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions,
					testing2.WithTriggerChannelReady(),
					testing2.WithFilterDeploymentAvailable(),
					testing2.WithIngressDeploymentAvailable(),
					testing2.WithBrokerAddress(fmt.Sprintf("%s.%s.svc.%s", ingressServiceName, testNS, utils.GetClusterDomainName())),
					testing2.WithIngressChannelFailed("ChannelFailure", "inducing failure for create channels")),
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
				testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions),
				// Use the channel name to avoid conflicting with the ingress one.
				testing2.NewChannel("filter-channel", testNS,
					testing2.WithChannelGenerateName(channelGenerateName),
					testing2.WithChannelLabels(TriggerChannelLabels(brokerName)),
					testing2.WithChannelOwnerReferences(ownerReferences()),
					testing2.WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					testing2.WithChannelReady,
					testing2.WithChannelAddress(triggerChannelHostname)),
				testing2.NewDeployment(filterDeploymentName, testNS,
					testing2.WithDeploymentOwnerReferences(ownerReferences()),
					testing2.WithDeploymentLabels(resources.FilterLabels(brokerName)),
					testing2.WithDeploymentAnnotations(annotations()),
					testing2.WithDeploymentServiceAccount(filterSA),
					testing2.WithDeploymentContainer(filterContainerName, filterImage, envVars(filterContainerName), nil)),
				testing2.NewService(filterServiceName, testNS,
					testing2.WithServiceOwnerReferences(ownerReferences()),
					testing2.WithServiceLabels(resources.FilterLabels(brokerName)),
					testing2.WithServicePorts(servicePorts(filterContainerName, 8080))),
				testing2.NewDeployment(ingressDeploymentName, testNS,
					testing2.WithDeploymentOwnerReferences(ownerReferences()),
					testing2.WithDeploymentLabels(resources.IngressLabels(brokerName)),
					testing2.WithDeploymentAnnotations(annotations()),
					testing2.WithDeploymentServiceAccount(ingressSA),
					testing2.WithDeploymentContainer(ingressContainerName, ingressImage, envVars(ingressContainerName), containerPorts(8080))),
				testing2.NewService(ingressServiceName, testNS,
					testing2.WithServiceOwnerReferences(ownerReferences()),
					testing2.WithServiceLabels(resources.IngressLabels(brokerName)),
					testing2.WithServicePorts(servicePorts(ingressContainerName, 8080))),
				// Use the channel name to avoid conflicting with the filter one.
				testing2.NewChannel("ingress-channel", testNS,
					testing2.WithChannelGenerateName(channelGenerateName),
					testing2.WithChannelLabels(IngressChannelLabels(brokerName)),
					testing2.WithChannelOwnerReferences(ownerReferences()),
					testing2.WithChannelProvisioner(provisionerGVK, "my-provisioner"),
					testing2.WithChannelReady,
					testing2.WithChannelAddress(ingressChannelHostname)),
			},
			WantCreates: []metav1.Object{
				testing2.NewSubscription("", testNS,
					testing2.WithSubscriptionGenerateName(ingressSubscriptionGenerateName),
					testing2.WithSubscriptionOwnerReferences(ownerReferences()),
					testing2.WithSubscriptionLabels(ingressSubscriptionLabels(brokerName)),
					testing2.WithSubscriptionChannel(channelGVK, "ingress-channel"),
					testing2.WithSubscriptionSubscriberRef(serviceGVK, ingressServiceName)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions,
					testing2.WithTriggerChannelReady(),
					testing2.WithFilterDeploymentAvailable(),
					testing2.WithIngressDeploymentAvailable(),
					testing2.WithBrokerAddress(fmt.Sprintf("%s.%s.svc.%s", ingressServiceName, testNS, utils.GetClusterDomainName())),
					testing2.WithBrokerIngressChannelReady(),
					testing2.WithBrokerIngressSubscriptionFailed("SubscriptionFailure", "inducing failure for create subscriptions"),
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
		//	{
		//		Name:   "Subscription.Delete error",
		//		Scheme: scheme.Scheme,
		//		InitialState: []runtime.Object{
		//			makeBroker(),
		//			makeTriggerChannel(),
		//			makeIngressChannel(),
		//			makeDifferentSubscription(),
		//		},
		//		Mocks: controllertesting.Mocks{
		//			MockDeletes: []controllertesting.MockDelete{
		//				func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
		//					if _, ok := obj.(*v1alpha1.Subscription); ok {
		//						return controllertesting.Handled, errors.New("test error deleting Subscription")
		//					}
		//					return controllertesting.Unhandled, nil
		//				},
		//			},
		//		},
		//		WantEvent:  []corev1.Event{events[ingressSubscriptionDeleteFailed], events[brokerReconcileError]},
		//		WantErrMsg: "test error deleting Subscription",
		//	},
		//	{
		//		Name:   "Subscription.Create error when recreating",
		//		Scheme: scheme.Scheme,
		//		InitialState: []runtime.Object{
		//			makeBroker(),
		//			makeTriggerChannel(),
		//			makeIngressChannel(),
		//			makeDifferentSubscription(),
		//		},
		//		Mocks: controllertesting.Mocks{
		//			MockCreates: []controllertesting.MockCreate{
		//				func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
		//					if _, ok := obj.(*v1alpha1.Subscription); ok {
		//						return controllertesting.Handled, errors.New("test error creating Subscription")
		//					}
		//					return controllertesting.Unhandled, nil
		//				},
		//			},
		//		},
		//		WantEvent:  []corev1.Event{events[ingressSubscriptionCreateFailed], events[brokerReconcileError]},
		//		WantErrMsg: "test error creating Subscription",
		//	},
		//	{
		//		Name:   "Broker.Get for status update fails",
		//		Scheme: scheme.Scheme,
		//		InitialState: []runtime.Object{
		//			makeBroker(),
		//			makeTriggerChannel(),
		//			makeIngressChannel(),
		//		},
		//		Mocks: controllertesting.Mocks{
		//			MockGets: []controllertesting.MockGet{
		//				// The first Get works.
		//				func(innerClient client.Client, ctx context.Context, key client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
		//					if _, ok := obj.(*v1alpha1.Broker); ok {
		//						return controllertesting.Handled, innerClient.Get(ctx, key, obj)
		//					}
		//					return controllertesting.Unhandled, nil
		//				},
		//				// The second Get fails.
		//				func(_ client.Client, _ context.Context, _ client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
		//					if _, ok := obj.(*v1alpha1.Broker); ok {
		//						return controllertesting.Handled, errors.New("test error getting the Broker for status update")
		//					}
		//					return controllertesting.Unhandled, nil
		//				},
		//			},
		//		},
		//		WantErrMsg: "test error getting the Broker for status update",
		//		WantEvent:  []corev1.Event{events[brokerUpdateStatusFailed]},
		//	},
		//	{
		//		Name:   "Broker.Status.Update error",
		//		Scheme: scheme.Scheme,
		//		InitialState: []runtime.Object{
		//			makeBroker(),
		//			makeTriggerChannel(),
		//			makeIngressChannel(),
		//		},
		//		Mocks: controllertesting.Mocks{
		//			MockStatusUpdates: []controllertesting.MockStatusUpdate{
		//				func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
		//					if _, ok := obj.(*v1alpha1.Broker); ok {
		//						return controllertesting.Handled, errors.New("test error updating the Broker status")
		//					}
		//					return controllertesting.Unhandled, nil
		//				},
		//			},
		//		},
		//		WantErrMsg: "test error updating the Broker status",
		//		WantEvent:  []corev1.Event{events[brokerUpdateStatusFailed]},
		//	},
		//	{
		//		Name:   "Successful reconcile",
		//		Scheme: scheme.Scheme,
		//		InitialState: []runtime.Object{
		//			makeBroker(),
		//			// The Channel needs to be addressable for the reconcile to succeed.
		//			makeTriggerChannel(),
		//			makeIngressChannel(),
		//			makeTestSubscription(),
		//		},
		//		Mocks: controllertesting.Mocks{
		//			MockLists: []controllertesting.MockList{
		//				// Controller Runtime's fake client totally ignores the opts.LabelSelector, so
		//				// picks up the Trigger Channel while listing the Ingress Channel. Use a mock to
		//				// force the correct behavior.
		//				func(innerClient client.Client, ctx context.Context, opts *client.ListOptions, list runtime.Object) (handled controllertesting.MockHandled, e error) {
		//					if cl, ok := list.(*v1alpha1.ChannelList); ok {
		//						// Only match the Ingress Channel labels.
		//						ls := labels.FormatLabels(IngressChannelLabels(makeBroker()))
		//						l, _ := labels.ConvertSelectorToLabelsMap(ls)
		//						if opts.LabelSelector.Matches(l) {
		//							cl.Items = append(cl.Items, *makeIngressChannel())
		//							return controllertesting.Handled, nil
		//						}
		//					}
		//					return controllertesting.Unhandled, nil
		//				},
		//			},
		//		},
		//		WantPresent: []runtime.Object{
		//			makeReadyBroker(),
		//			// TODO Uncomment makeTriggerChannel() when our test framework handles generateName.
		//			// makeTriggerChannel(),
		//			makeFilterDeployment(),
		//			makeFilterService(),
		//			makeIngressDeployment(),
		//			makeIngressService(),
		//			// TODO Uncomment makeIngressChannel() when our test framework handles generateName.
		//			// makeIngressChannel(),
		//			makeTestSubscription(),
		//		},
		//		WantEvent: []corev1.Event{
		//			events[brokerReadinessChanged],
		//		},
		//	},
	}

	defer logtesting.ClearAll()
	table.Test(t, testing2.MakeFactory(func(listers *testing2.Listers, opt reconciler.Options) controller.Reconciler {
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
	}))
}

//func makeReadyBroker() *v1alpha1.Broker {
//	b := makeBroker()
//	b.Status.InitializeConditions()
//	b.Status.PropagateIngressDeploymentAvailability(makeAvailableDeployment())
//	b.Status.PropagateTriggerChannelReadiness(makeReadyChannelStatus())
//	b.Status.PropagateIngressChannelReadiness(makeReadyChannelStatus())
//	b.Status.PropagateFilterDeploymentAvailability(makeAvailableDeployment())
//	b.Status.SetAddress(fmt.Sprintf("%s-broker.%s.svc.%s", brokerName, testNS, utils.GetClusterDomainName()))
//	b.Status.PropagateIngressSubscriptionReadiness(makeReadySubscriptionStatus())
//	return b
//}

//func makeDeletingBroker() *v1alpha1.Broker {
//	b := makeReadyBroker()
//	b.DeletionTimestamp = &deletionTime
//	return b
//}

//func makeTriggerChannel() *v1alpha1.Channel {
//	c := &v1alpha1.Channel{
//		ObjectMeta: metav1.ObjectMeta{
//			Namespace:    testNS,
//			GenerateName: fmt.Sprintf("%s-broker-", brokerName),
//			Labels: map[string]string{
//				"eventing.knative.dev/broker":           brokerName,
//				"eventing.knative.dev/brokerEverything": "true",
//			},
//			OwnerReferences: ownerReferences(),
//		},
//		Spec: v1alpha1.ChannelSpec{
//			Provisioner: channelProvisioner,
//		},
//	}
//	c.Status.MarkProvisionerInstalled()
//	c.Status.MarkProvisioned()
//	c.Status.SetAddress(triggerChannelHostname)
//	return c
//}

//func makeNonAddressableTriggerChannel() *v1alpha1.Channel {
//	c := makeTriggerChannel()
//	c.Status.Address = duckv1alpha1.Addressable{}
//	return c
//}

//func makeDifferentTriggerChannel() *v1alpha1.Channel {
//	c := makeTriggerChannel()
//	c.Spec.Provisioner.Name = "some-other-provisioner"
//	return c
//}

//func makeIngressChannel() *v1alpha1.Channel {
//	c := &v1alpha1.Channel{
//		ObjectMeta: metav1.ObjectMeta{
//			Namespace:    testNS,
//			GenerateName: fmt.Sprintf("%s-broker-ingress-", brokerName),
//			// The Fake library doesn't understand GenerateName, so give this a name so it doesn't
//			// collide with the Trigger Channel.
//			Name: ingressChannelName,
//			Labels: map[string]string{
//				"eventing.knative.dev/broker":        brokerName,
//				"eventing.knative.dev/brokerIngress": "true",
//			},
//			OwnerReferences: []metav1.OwnerReference{
//				ownerReferences(),
//			},
//		},
//		Spec: v1alpha1.ChannelSpec{
//			Provisioner: channelProvisioner(),
//		},
//	}
//	c.Status.MarkProvisionerInstalled()
//	c.Status.MarkProvisioned()
//	c.Status.SetAddress(ingressChannelHostname)
//	return c
//}

//func makeNonAddressableIngressChannel() *v1alpha1.Channel {
//	c := makeIngressChannel()
//	c.Status.Address = duckv1alpha1.Addressable{}
//	return c
//}

//func makeDifferentIngressChannel() *v1alpha1.Channel {
//	c := makeIngressChannel()
//	c.Spec.Provisioner.Name = "some-other-provisioner"
//	return c
//}

//func makeDifferentFilterDeployment() *appsv1.Deployment {
//	d := makeFilterDeployment()
//	d.Spec.Template.Spec.Containers[0].Image = "some-other-image"
//	return d
//}

//func makeFilterService() *corev1.Service {
//	svc := resources.MakeFilterService(makeBroker())
//	return svc
//}
//
//func makeDifferentFilterService() *corev1.Service {
//	s := makeFilterService()
//	s.Spec.Selector["eventing.knative.dev/broker"] = "some-other-value"
//	return s
//}
//
//func makeIngressDeployment() *appsv1.Deployment {
//	d := resources.MakeIngress(&resources.IngressArgs{
//		Broker:             makeBroker(),
//		Image:              ingressImage,
//		ServiceAccountName: ingressSA,
//		ChannelAddress:     triggerChannelHostname,
//	})
//	d.TypeMeta = metav1.TypeMeta{
//		APIVersion: "apps/v1",
//		Kind:       "Deployment",
//	}
//	return d
//}

//func makeDifferentIngressDeployment() *appsv1.Deployment {
//	d := makeIngressDeployment()
//	d.Spec.Template.Spec.Containers[0].Image = "some-other-image"
//	return d
//}
//
//func makeIngressService() *corev1.Service {
//	svc := resources.MakeIngressService(makeBroker())
//	return svc
//}
//
//func makeDifferentIngressService() *corev1.Service {
//	s := makeIngressService()
//	s.Spec.Selector["eventing.knative.dev/broker"] = "some-other-value"
//	return s
//}
//
//func makeTestSubscription() *v1alpha1.Subscription {
//	s := &v1alpha1.Subscription{
//		TypeMeta: metav1.TypeMeta{
//			APIVersion: "eventing.knative.dev/v1alpha1",
//			Kind:       "Subscription",
//		},
//		ObjectMeta: metav1.ObjectMeta{
//			Namespace:    testNS,
//			GenerateName: fmt.Sprintf("internal-ingress-%s-", brokerName),
//			Labels: map[string]string{
//				"eventing.knative.dev/broker":        brokerName,
//				"eventing.knative.dev/brokerIngress": "true",
//			},
//			OwnerReferences: ownerReferences(),
//		},
//		Spec: v1alpha1.SubscriptionSpec{
//			Channel: corev1.ObjectReference{
//				APIVersion: v1alpha1.SchemeGroupVersion.String(),
//				Kind:       "Channel",
//				Name:       ingressChannelName,
//			},
//			Subscriber: &v1alpha1.SubscriberSpec{
//				Ref: &corev1.ObjectReference{
//					APIVersion: "v1",
//					Kind:       "Service",
//					Name:       makeIngressService().Name,
//				},
//			},
//		},
//	}
//	s.Status = *v1alpha1.TestHelper.ReadySubscriptionStatus()
//	return s
//}

//func makeDifferentSubscription() *v1alpha1.Subscription {
//	s := makeTestSubscription()
//	s.Spec.Subscriber.Ref = nil
//	url := "http://example.com/"
//	s.Spec.Subscriber.URI = &url
//	return s
//}

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

// TODO remove this once we get rid of istio.
func annotations() map[string]string {
	return map[string]string{
		"sidecar.istio.io/inject": "true",
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

func makeFilterDeployment(broker *v1alpha1.Broker) *appsv1.Deployment {
	d := resources.MakeFilterDeployment(&resources.FilterArgs{
		Broker:             broker,
		Image:              filterImage,
		ServiceAccountName: filterSA,
	})
	d.TypeMeta = metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	}
	return d
}

func makeAvailableDeployment() *v1.Deployment {
	d := &v1.Deployment{}
	d.Name = "deployment-name"
	d.Status.Conditions = []v1.DeploymentCondition{
		{
			Type:   v1.DeploymentAvailable,
			Status: "True",
		},
	}
	return d
}

func makeReadyChannelStatus() *v1alpha1.ChannelStatus {
	return v1alpha1.TestHelper.ReadyChannelStatus()
}

func makeReadySubscriptionStatus() *v1alpha1.SubscriptionStatus {
	return v1alpha1.TestHelper.ReadySubscriptionStatus()
}
