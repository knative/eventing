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
	"k8s.io/apimachinery/pkg/runtime"
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

	filterContainerName = "filter"
)

var (
	trueVal = true

	testKey             = fmt.Sprintf("%s/%s", testNS, brokerName)
	channelGenerateName = fmt.Sprintf("%s-broker-", brokerName)

	triggerChannelHostname = fmt.Sprintf("foo.bar.svc.%s", utils.GetClusterDomainName())
	ingressChannelHostname = fmt.Sprintf("baz.qux.svc.%s", utils.GetClusterDomainName())

	filterDeploymentName = fmt.Sprintf("%s-broker-filter", brokerName)
	filterServiceName    = fmt.Sprintf("%s-broker-filter", brokerName)
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
					testing2.WithChannelProvisioner(channelProvisioner("my-provisioner"))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewBroker(brokerName, testNS,
					testing2.WithInitBrokerConditions,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.MarkTriggerChannelFailed("ChannelFailure", "%v", "inducing failure for create channels")),
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
					testing2.WithChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithChannelAddress("")),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions,
					testing2.MarkTriggerChannelFailed("NoAddress", "%v", "Channel does not have an address.")),
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
					testing2.WithChannelProvisioner(channelProvisioner("my-provisioner")),
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
					testing2.WithDeploymentContainer(filterContainerName, filterImage, envVars())),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions,
					testing2.PropagateTriggerChannelReadiness(channelStatus()),
					testing2.MarkFilterFailed("DeploymentFailure", "%v", "inducing failure for create deployments")),
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
					testing2.WithChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithChannelAddress(triggerChannelHostname)),
				testing2.NewDeployment(filterDeploymentName, testNS,
					testing2.WithDeploymentOwnerReferences(ownerReferences()),
					testing2.WithDeploymentLabels(resources.FilterLabels(brokerName)),
					testing2.WithDeploymentAnnotations(annotations()),
					testing2.WithDeploymentServiceAccount(filterSA),
					testing2.WithDeploymentContainer(filterContainerName, "some-other-image", envVars())),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "deployments"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions,
					testing2.PropagateTriggerChannelReadiness(channelStatus()),
					testing2.MarkFilterFailed("DeploymentFailure", "%v", "inducing failure for update deployments")),
			}},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewDeployment(filterDeploymentName, testNS,
					testing2.WithDeploymentOwnerReferences(ownerReferences()),
					testing2.WithDeploymentLabels(resources.FilterLabels(brokerName)),
					testing2.WithDeploymentAnnotations(annotations()),
					testing2.WithDeploymentServiceAccount(filterSA),
					testing2.WithDeploymentContainer(filterContainerName, filterImage, envVars())),
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
					testing2.WithChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithChannelAddress(triggerChannelHostname)),
				testing2.NewDeployment(filterDeploymentName, testNS,
					testing2.WithDeploymentOwnerReferences(ownerReferences()),
					testing2.WithDeploymentLabels(resources.FilterLabels(brokerName)),
					testing2.WithDeploymentAnnotations(annotations()),
					testing2.WithDeploymentServiceAccount(filterSA),
					testing2.WithDeploymentContainer(filterContainerName, filterImage, envVars())),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "services"),
			},
			WantCreates: []metav1.Object{
				testing2.NewService(filterServiceName, testNS,
					testing2.WithServiceOwnerReferences(ownerReferences()),
					testing2.WithServiceLabels(resources.FilterLabels(brokerName)),
					testing2.WithServicePorts(ports(8080))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions,
					testing2.PropagateTriggerChannelReadiness(channelStatus()),
					testing2.MarkFilterFailed("ServiceFailure", "%v", "inducing failure for create services")),
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
					testing2.WithChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithChannelAddress(triggerChannelHostname)),
				testing2.NewDeployment(filterDeploymentName, testNS,
					testing2.WithDeploymentOwnerReferences(ownerReferences()),
					testing2.WithDeploymentLabels(resources.FilterLabels(brokerName)),
					testing2.WithDeploymentAnnotations(annotations()),
					testing2.WithDeploymentServiceAccount(filterSA),
					testing2.WithDeploymentContainer(filterContainerName, filterImage, envVars())),
				testing2.NewService(filterServiceName, testNS,
					testing2.WithServiceOwnerReferences(ownerReferences()),
					testing2.WithServiceLabels(resources.FilterLabels(brokerName)),
					testing2.WithServicePorts(ports(9090))),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "services"),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewService(filterServiceName, testNS,
					testing2.WithServiceOwnerReferences(ownerReferences()),
					testing2.WithServiceLabels(resources.FilterLabels(brokerName)),
					testing2.WithServicePorts(ports(8080))),
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: testing2.NewBroker(brokerName, testNS,
					testing2.WithBrokerChannelProvisioner(channelProvisioner("my-provisioner")),
					testing2.WithInitBrokerConditions,
					testing2.PropagateTriggerChannelReadiness(channelStatus()),
					testing2.MarkFilterFailed("ServiceFailure", "%v", "inducing failure for update services")),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, brokerReconcileError, "Broker reconcile error: %v", "inducing failure for update services"),
			},
			WantErr: true,
		},
		//	{
		//		Name:   "Ingress Deployment.Get error",
		//		Scheme: scheme.Scheme,
		//		InitialState: []runtime.Object{
		//			makeBroker(),
		//			makeTriggerChannel(),
		//		},
		//		Mocks: controllertesting.Mocks{
		//			MockGets: []controllertesting.MockGet{
		//				func(_ client.Client, _ context.Context, key client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
		//					if _, ok := obj.(*appsv1.Deployment); ok {
		//						if strings.Contains(key.Name, "ingress") {
		//							return controllertesting.Handled, errors.New("test error getting ingress Deployment")
		//						}
		//					}
		//					return controllertesting.Unhandled, nil
		//				},
		//			},
		//		},
		//		WantEvent:  []corev1.Event{events[brokerReconcileError]},
		//		WantErrMsg: "test error getting ingress Deployment",
		//	},
		//	{
		//		Name:   "Ingress Deployment.Create error",
		//		Scheme: scheme.Scheme,
		//		InitialState: []runtime.Object{
		//			makeBroker(),
		//			makeTriggerChannel(),
		//		},
		//		Mocks: controllertesting.Mocks{
		//			MockCreates: []controllertesting.MockCreate{
		//				func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
		//					if d, ok := obj.(*appsv1.Deployment); ok {
		//						if d.Labels["eventing.knative.dev/brokerRole"] == "ingress" {
		//							return controllertesting.Handled, errors.New("test error creating ingress Deployment")
		//						}
		//					}
		//					return controllertesting.Unhandled, nil
		//				},
		//			},
		//		},
		//		WantEvent:  []corev1.Event{events[brokerReconcileError]},
		//		WantErrMsg: "test error creating ingress Deployment",
		//	},
		//	{
		//		Name:   "Ingress Deployment.Update error",
		//		Scheme: scheme.Scheme,
		//		InitialState: []runtime.Object{
		//			makeBroker(),
		//			makeTriggerChannel(),
		//			makeDifferentIngressDeployment(),
		//		},
		//		Mocks: controllertesting.Mocks{
		//			MockUpdates: []controllertesting.MockUpdate{
		//				func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
		//					if d, ok := obj.(*appsv1.Deployment); ok {
		//						if d.Labels["eventing.knative.dev/brokerRole"] == "ingress" {
		//							return controllertesting.Handled, errors.New("test error updating ingress Deployment")
		//						}
		//					}
		//					return controllertesting.Unhandled, nil
		//				},
		//			},
		//		},
		//		WantEvent:  []corev1.Event{events[brokerReconcileError]},
		//		WantErrMsg: "test error updating ingress Deployment",
		//	},
		//	{
		//		Name:   "Ingress Service.Get error",
		//		Scheme: scheme.Scheme,
		//		InitialState: []runtime.Object{
		//			makeBroker(),
		//			makeTriggerChannel(),
		//		},
		//		Mocks: controllertesting.Mocks{
		//			MockGets: []controllertesting.MockGet{
		//				func(_ client.Client, _ context.Context, key client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
		//					if _, ok := obj.(*corev1.Service); ok {
		//						if key.Name == fmt.Sprintf("%s-broker", brokerName) {
		//							return controllertesting.Handled, errors.New("test error getting ingress Service")
		//						}
		//					}
		//					return controllertesting.Unhandled, nil
		//				},
		//			},
		//		},
		//		WantEvent:  []corev1.Event{events[brokerReconcileError]},
		//		WantErrMsg: "test error getting ingress Service",
		//	},
		//	{
		//		Name:   "Ingress Service.Create error",
		//		Scheme: scheme.Scheme,
		//		InitialState: []runtime.Object{
		//			makeBroker(),
		//			makeTriggerChannel(),
		//		},
		//		Mocks: controllertesting.Mocks{
		//			MockCreates: []controllertesting.MockCreate{
		//				func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
		//					if svc, ok := obj.(*corev1.Service); ok {
		//						if svc.Labels["eventing.knative.dev/brokerRole"] == "ingress" {
		//							return controllertesting.Handled, errors.New("test error creating ingress Service")
		//						}
		//					}
		//					return controllertesting.Unhandled, nil
		//				},
		//			},
		//		},
		//		WantEvent:  []corev1.Event{events[brokerReconcileError]},
		//		WantErrMsg: "test error creating ingress Service",
		//	},
		//	{
		//		Name:   "Ingress Service.Update error",
		//		Scheme: scheme.Scheme,
		//		InitialState: []runtime.Object{
		//			makeBroker(),
		//			makeTriggerChannel(),
		//			makeDifferentIngressService(),
		//		},
		//		Mocks: controllertesting.Mocks{
		//			MockUpdates: []controllertesting.MockUpdate{
		//				func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
		//					if svc, ok := obj.(*corev1.Service); ok {
		//						if svc.Labels["eventing.knative.dev/brokerRole"] == "ingress" {
		//							return controllertesting.Handled, errors.New("test error updating ingress Service")
		//						}
		//					}
		//					return controllertesting.Unhandled, nil
		//				},
		//			},
		//		},
		//		WantEvent:  []corev1.Event{events[brokerReconcileError]},
		//		WantErrMsg: "test error updating ingress Service",
		//	},
		//	{
		//		Name:   "Ingress Channel.List error",
		//		Scheme: scheme.Scheme,
		//		InitialState: []runtime.Object{
		//			makeBroker(),
		//			makeTriggerChannel(),
		//		},
		//		Mocks: controllertesting.Mocks{
		//			MockLists: []controllertesting.MockList{
		//				func(_ client.Client, _ context.Context, opts *client.ListOptions, list runtime.Object) (controllertesting.MockHandled, error) {
		//					// Only match the Ingress Channel labels.
		//					ls := labels.FormatLabels(IngressChannelLabels(makeBroker()))
		//					l, _ := labels.ConvertSelectorToLabelsMap(ls)
		//
		//					if _, ok := list.(*v1alpha1.ChannelList); ok && opts.LabelSelector.Matches(l) {
		//						return controllertesting.Handled, errors.New("test error getting Ingress Channel")
		//					}
		//					return controllertesting.Unhandled, nil
		//				},
		//			},
		//		},
		//		WantEvent:  []corev1.Event{events[brokerReconcileError]},
		//		WantErrMsg: "test error getting Ingress Channel",
		//	},
		//	{
		//		Name:   "Ingress Channel.Create error",
		//		Scheme: scheme.Scheme,
		//		InitialState: []runtime.Object{
		//			makeBroker(),
		//			makeTriggerChannel(),
		//		},
		//		Mocks: controllertesting.Mocks{
		//			MockLists: []controllertesting.MockList{
		//				// Controller Runtime's fake client totally ignores the opts.LabelSelector, so
		//				// picks up the Trigger Channel while listing the Ingress Channel. Use a mock to
		//				// force the correct behavior.
		//				func(innerClient client.Client, ctx context.Context, opts *client.ListOptions, list runtime.Object) (handled controllertesting.MockHandled, e error) {
		//					if _, ok := list.(*v1alpha1.ChannelList); ok {
		//						// Only match the Ingress Channel labels.
		//						ls := labels.FormatLabels(IngressChannelLabels(makeBroker()))
		//						l, _ := labels.ConvertSelectorToLabelsMap(ls)
		//						if opts.LabelSelector.Matches(l) {
		//							return controllertesting.Handled, nil
		//						}
		//					}
		//					return controllertesting.Unhandled, nil
		//				},
		//			},
		//			MockCreates: []controllertesting.MockCreate{
		//				func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
		//					if c, ok := obj.(*v1alpha1.Channel); ok {
		//						if cmp.Equal(c.Labels, IngressChannelLabels(makeBroker())) {
		//							return controllertesting.Handled, errors.New("test error creating Ingress Channel")
		//						}
		//					}
		//					return controllertesting.Unhandled, nil
		//				},
		//			},
		//		},
		//		WantEvent:  []corev1.Event{events[brokerReconcileError]},
		//		WantErrMsg: "test error creating Ingress Channel",
		//	},
		//	{
		//		Name:   "Ingress Channel is different than expected",
		//		Scheme: scheme.Scheme,
		//		InitialState: []runtime.Object{
		//			makeBroker(),
		//			makeTriggerChannel(),
		//			makeDifferentIngressChannel(),
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
		//							cl.Items = append(cl.Items, *makeDifferentIngressChannel())
		//							return controllertesting.Handled, nil
		//						}
		//					}
		//					return controllertesting.Unhandled, nil
		//				},
		//			},
		//		},
		//		WantPresent: []runtime.Object{
		//			// This is special because the Channel is not updated, unlike most things that
		//			// differ from expected.
		//			// TODO uncomment the following line once our test framework supports searching for
		//			// GenerateName.
		//			// makeDifferentIngressChannel(),
		//		},
		//	},
		//	{
		//		Name:   "Ingress Channel is not yet Addressable",
		//		Scheme: scheme.Scheme,
		//		InitialState: []runtime.Object{
		//			makeBroker(),
		//			makeTriggerChannel(),
		//			makeNonAddressableIngressChannel(),
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
		//							cl.Items = append(cl.Items, *makeNonAddressableIngressChannel())
		//							return controllertesting.Handled, nil
		//						}
		//					}
		//					return controllertesting.Unhandled, nil
		//				},
		//			},
		//		},
		//	},
		//	{
		//		Name:   "Subscription.List error",
		//		Scheme: scheme.Scheme,
		//		InitialState: []runtime.Object{
		//			makeBroker(),
		//			makeTriggerChannel(),
		//			makeIngressChannel(),
		//		},
		//		Mocks: controllertesting.Mocks{
		//			MockLists: []controllertesting.MockList{
		//				func(_ client.Client, _ context.Context, opts *client.ListOptions, list runtime.Object) (controllertesting.MockHandled, error) {
		//					if _, ok := list.(*v1alpha1.SubscriptionList); ok {
		//						return controllertesting.Handled, errors.New("test error getting Subscription")
		//					}
		//					return controllertesting.Unhandled, nil
		//				},
		//			},
		//		},
		//		WantEvent:  []corev1.Event{events[brokerReconcileError]},
		//		WantErrMsg: "test error getting Subscription",
		//	},
		//	{
		//		Name:   "Subscription.Create error",
		//		Scheme: scheme.Scheme,
		//		InitialState: []runtime.Object{
		//			makeBroker(),
		//			makeTriggerChannel(),
		//			makeIngressChannel(),
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
		//		WantEvent:  []corev1.Event{events[brokerReconcileError]},
		//		WantErrMsg: "test error creating Subscription",
		//	},
		//	{
		//		Name:   "Subscription is different than expected",
		//		Scheme: scheme.Scheme,
		//		InitialState: []runtime.Object{
		//			makeBroker(),
		//			makeTriggerChannel(),
		//			makeIngressChannel(),
		//		},
		//		WantPresent: []runtime.Object{
		//			// This is special because the Channel is not updated, unlike most things that
		//			// differ from expected.
		//			// TODO uncomment the following line once our test framework supports searching for
		//			// GenerateName.
		//			// makeDifferentSubscription(),
		//		},
		//	},
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
//	return nil
//	//b := makeReadyBroker()
//	//b.DeletionTimestamp = &deletionTime
//	//return b
//}
//
//func makeTriggerChannel() *v1alpha1.Channel {
//	c := &v1alpha1.Channel{
//		ObjectMeta: metav1.ObjectMeta{
//			Namespace:    testNS,
//			GenerateName: fmt.Sprintf("%s-broker-", brokerName),
//			Labels: map[string]string{
//				"eventing.knative.dev/broker":           brokerName,
//				"eventing.knative.dev/brokerEverything": "true",
//			},
//			OwnerReferences: []metav1.OwnerReference{
//				getOwnerReference(),
//			},
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
//
//func makeNonAddressableTriggerChannel() *v1alpha1.Channel {
//	c := makeTriggerChannel()
//	c.Status.Address = duckv1alpha1.Addressable{}
//	return c
//}
//
//func makeDifferentTriggerChannel() *v1alpha1.Channel {
//	c := makeTriggerChannel()
//	c.Spec.Provisioner.Name = "some-other-provisioner"
//	return c
//}
//
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
//				getOwnerReference(),
//			},
//		},
//		Spec: v1alpha1.ChannelSpec{
//			Provisioner: channelProvisioner,
//		},
//	}
//	c.Status.MarkProvisionerInstalled()
//	c.Status.MarkProvisioned()
//	c.Status.SetAddress(ingressChannelHostname)
//	return c
//}
//
//func makeNonAddressableIngressChannel() *v1alpha1.Channel {
//	c := makeIngressChannel()
//	c.Status.Address = duckv1alpha1.Addressable{}
//	return c
//}
//
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
//	svc.TypeMeta = metav1.TypeMeta{
//		APIVersion: "v1",
//		Kind:       "Service",
//	}
//	return svc
//}

//func makeDifferentFilterService() *corev1.Service {
//	s := makeFilterService()
//	s.Spec.Selector["eventing.knative.dev/broker"] = "some-other-value"
//	return s
//}

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

//func makeIngressService() *corev1.Service {
//	svc := resources.MakeIngressService(makeBroker())
//	svc.TypeMeta = metav1.TypeMeta{
//		APIVersion: "v1",
//		Kind:       "Service",
//	}
//	return svc
//}

//func makeDifferentIngressService() *corev1.Service {
//	s := makeIngressService()
//	s.Spec.Selector["eventing.knative.dev/broker"] = "some-other-value"
//	return s
//}

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
//			OwnerReferences: []metav1.OwnerReference{
//				getOwnerReference(),
//			},
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
//	s.Status.MarkChannelReady()
//	s.Status.MarkReferencesResolved()
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

func channelStatus() *v1alpha1.ChannelStatus {
	return &v1alpha1.ChannelStatus{}
}

// TODO remove this once we get rid of istio.
func annotations() map[string]string {
	return map[string]string{
		"sidecar.istio.io/inject": "true",
	}
}

func envVars() []corev1.EnvVar {
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
}

func ports(internal int) []corev1.ServicePort {
	return []corev1.ServicePort{
		{
			Name:       "http",
			Port:       80,
			TargetPort: intstr.FromInt(internal),
		},
	}
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
	cs := &v1alpha1.ChannelStatus{}
	cs.MarkProvisionerInstalled()
	cs.MarkProvisioned()
	cs.SetAddress("foo")
	return cs
}

func makeReadySubscriptionStatus() *v1alpha1.SubscriptionStatus {
	ss := &v1alpha1.SubscriptionStatus{}
	ss.MarkChannelReady()
	ss.MarkReferencesResolved()
	return ss
}
