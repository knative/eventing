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

package trigger

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	fakeclientset "github.com/knative/eventing/pkg/client/clientset/versioned/fake"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"github.com/knative/eventing/pkg/reconciler"
	brokerresources "github.com/knative/eventing/pkg/reconciler/broker/resources"
	reconciletesting "github.com/knative/eventing/pkg/reconciler/testing"
	"github.com/knative/eventing/pkg/reconciler/trigger/resources"
	"github.com/knative/eventing/pkg/utils"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	logtesting "github.com/knative/pkg/logging/testing"
	. "github.com/knative/pkg/reconciler/testing"
	"github.com/knative/pkg/tracker"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
)

const (
	testNS      = "test-namespace"
	triggerName = "test-trigger"
	triggerUID  = "test-trigger-uid"
	brokerName  = "test-broker"

	subscriberAPIVersion = "v1"
	subscriberKind       = "Service"
	subscriberName       = "subscriberName"
	subscriberURI        = "http://example.com/subscriber"
)

var (
	trueVal = true
)

func init() {
	// Add types to scheme
	_ = v1alpha1.AddToScheme(scheme.Scheme)
	_ = duckv1alpha1.AddToScheme(scheme.Scheme)
}

func TestNewController(t *testing.T) {
	kubeClient := fakekubeclientset.NewSimpleClientset()
	eventingClient := fakeclientset.NewSimpleClientset()

	// Create informer factories with fake clients. The second parameter sets the
	// resync period to zero, disabling it.
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
	eventingInformerFactory := informers.NewSharedInformerFactory(eventingClient, 0)

	// Eventing
	triggerInformer := eventingInformerFactory.Eventing().V1alpha1().Triggers()
	channelInformer := eventingInformerFactory.Eventing().V1alpha1().Channels()
	subscriptionInformer := eventingInformerFactory.Eventing().V1alpha1().Subscriptions()
	brokerInformer := eventingInformerFactory.Eventing().V1alpha1().Brokers()

	// Kube
	serviceInformer := kubeInformerFactory.Core().V1().Services()

	// Duck
	addressableInformer := &fakeAddressableInformer{}

	c := NewController(
		reconciler.Options{
			KubeClientSet:     kubeClient,
			EventingClientSet: eventingClient,
			Logger:            logtesting.TestLogger(t),
		},
		triggerInformer,
		channelInformer,
		subscriptionInformer,
		brokerInformer,
		serviceInformer,
		addressableInformer)

	if c == nil {
		t.Fatalf("Failed to create with NewController")
	}
}

type fakeAddressableInformer struct{}

func (*fakeAddressableInformer) TrackInNamespace(tracker.Interface, metav1.Object) func(corev1.ObjectReference) error {
	return func(corev1.ObjectReference) error { return nil }
}

func TestAllCases(t *testing.T) {
	triggerKey := testNS + "/" + triggerName
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
			//					reconciletesting.WithTriggerUID(triggerUID),
			//			},
			//			Key:     "foo/incomplete",
			//			WantErr: true,
			//			WantEvents: []string{
			//				Eventf(corev1.EventTypeWarning, "ChannelReferenceFetchFailed", "Failed to validate spec.channel exists: s \"\" not found"),
			//			},
		}, {
			Name: "Broker not found",
			Key:  triggerKey,
			Objects: []runtime.Object{
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI)),
			},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconciliation failed: broker.eventing.knative.dev \"test-broker\" not found"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerFailed("DoesNotExist", "Broker does not exist"),
				),
			}},
		}, {
			Name: "Broker get failure",
			Key:  triggerKey,
			Objects: []runtime.Object{
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI)),
			},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconciliation failed: broker.eventing.knative.dev \"test-broker\" not found"),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("get", "brokers"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerFailed("DoesNotExist", "Broker does not exist"),
				),
			}},
		}, {
			Name: "Trigger being deleted",
			Key:  triggerKey,
			Objects: []runtime.Object{
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerDeleted),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
		}, {
			Name: "No Broker Trigger Channel",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "TriggerChannelFailed", "Broker's Trigger channel not found"),
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconciliation failed: failed to find Broker's Trigger channel"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
				),
			}},
		}, {
			Name: "No Broker Ingress Channel",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeTriggerChannel(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "IngressChannelFailed", "Broker's Ingress channel not found"),
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconciliation failed: failed to find Broker's Ingress channel"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
				),
			}},
		}, {
			Name: "No Broker Filter Service",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeTriggerChannel(),
				makeIngressChannel(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "TriggerServiceFailed", "Broker's Filter service not found"),
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconciliation failed: failed to find Broker's Filter service"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
				),
			}},
		}, {
			Name: "Subscription create fail",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeTriggerChannel(),
				makeIngressChannel(),
				makeBrokerFilterService(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: true,
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "subscriptions"),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriptionCreateFailed", "Create Trigger's subscription failed: inducing failure for create subscriptions"),
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconciliation failed: inducing failure for create subscriptions")},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerNotSubscribed("NotSubscribed", "inducing failure for create subscriptions"),
					reconciletesting.WithTriggerStatusSubscriberURI(subscriberURI),
				),
			}},
			WantCreates: []metav1.Object{
				makeIngressSubscription(),
			},
		}, {
			Name: "Subscription delete fail",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeTriggerChannel(),
				makeIngressChannel(),
				makeBrokerFilterService(),
				makeDifferentReadySubscription(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: true,
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("delete", "subscriptions"),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriptionDeleteFailed", "Delete Trigger's subscription failed: inducing failure for delete subscriptions"),
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconciliation failed: inducing failure for delete subscriptions")},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerNotSubscribed("NotSubscribed", "inducing failure for delete subscriptions"),
					reconciletesting.WithTriggerStatusSubscriberURI(subscriberURI),
				),
			}},
			// Name being "" is NOT a bug. Because we use generate name, the object created
			// does not have a name...
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				Name: "",
			}},
		}, {
			Name: "Subscription create after delete fail",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeTriggerChannel(),
				makeIngressChannel(),
				makeBrokerFilterService(),
				makeDifferentReadySubscription(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: true,
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "subscriptions"),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SubscriptionCreateFailed", "Create Trigger's subscription failed: inducing failure for create subscriptions"),
				Eventf(corev1.EventTypeWarning, "TriggerReconcileFailed", "Trigger reconciliation failed: inducing failure for create subscriptions")},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerNotSubscribed("NotSubscribed", "inducing failure for create subscriptions"),
					reconciletesting.WithTriggerStatusSubscriberURI(subscriberURI),
				),
			}},
			// Name being "" is NOT a bug. Because we use generate name, the object created
			// does not have a name...
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				Name: "",
			}},
			WantCreates: []metav1.Object{
				makeIngressSubscription(),
			},
		}, {
			Name: "Subscription updated works",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeTriggerChannel(),
				makeIngressChannel(),
				makeBrokerFilterService(),
				makeDifferentReadySubscription(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerNotSubscribed("SubscriptionNotReady", "Subscription is not ready: nil"),
					reconciletesting.WithTriggerStatusSubscriberURI(subscriberURI),
				),
			}},
			// Name being "" is NOT a bug. Because we use generate name, the object created
			// does not have a name...
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				Name: "",
			}},
			WantCreates: []metav1.Object{
				makeIngressSubscription(),
			},
		}, {
			Name: "Subscription Created, not ready",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeTriggerChannel(),
				makeIngressChannel(),
				makeBrokerFilterService(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerNotSubscribed("SubscriptionNotReady", "Subscription is not ready: nil"),
					reconciletesting.WithTriggerStatusSubscriberURI(subscriberURI),
				),
			}},
			WantCreates: []metav1.Object{
				makeIngressSubscription(),
			},
		}, {
			Name: "Subscription not ready, trigger marked not ready",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeTriggerChannel(),
				makeIngressChannel(),
				makeBrokerFilterService(),
				makeNotReadySubscription(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerNotSubscribed("SubscriptionNotReady", "Subscription is not ready: nil"),
					reconciletesting.WithTriggerStatusSubscriberURI(subscriberURI),
				),
			}},
		}, {
			Name: "Subscription ready, trigger marked ready",
			Key:  triggerKey,
			Objects: []runtime.Object{
				makeReadyBroker(),
				makeTriggerChannel(),
				makeIngressChannel(),
				makeBrokerFilterService(),
				makeReadySubscription(),
				reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					reconciletesting.WithInitTriggerConditions,
				),
			},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "TriggerReconciled", "Trigger reconciled"),
				Eventf(corev1.EventTypeNormal, "TriggerReadinessChanged", `Trigger "test-trigger" became ready`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewTrigger(triggerName, testNS, brokerName,
					reconciletesting.WithTriggerUID(triggerUID),
					reconciletesting.WithTriggerSubscriberURI(subscriberURI),
					// The first reconciliation will initialize the status conditions.
					reconciletesting.WithInitTriggerConditions,
					reconciletesting.WithTriggerBrokerReady(),
					reconciletesting.WithTriggerSubscribed(),
					reconciletesting.WithTriggerStatusSubscriberURI(subscriberURI),
				),
			}},
		},
	}

	defer logtesting.ClearAll()

	table.Test(t, reconciletesting.MakeFactory(func(listers *reconciletesting.Listers, opt reconciler.Options) controller.Reconciler {
		return &Reconciler{
			Base:                reconciler.NewBase(opt, controllerAgentName),
			triggerLister:       listers.GetTriggerLister(),
			channelLister:       listers.GetChannelLister(),
			subscriptionLister:  listers.GetSubscriptionLister(),
			brokerLister:        listers.GetBrokerLister(),
			serviceLister:       listers.GetK8sServiceLister(),
			addressableInformer: &fakeAddressableInformer{},
			tracker:             tracker.New(func(string) {}, 0),
		}
	},
	false,
	))
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
				SourceAndType: &v1alpha1.TriggerFilterSourceAndType{
					Source: "Any",
					Type:   "Any",
				},
			},
			Subscriber: &v1alpha1.SubscriberSpec{
				Ref: &corev1.ObjectReference{
					Name:       subscriberName,
					Kind:       subscriberKind,
					APIVersion: subscriberAPIVersion,
				},
			},
		},
	}
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
		Spec: v1alpha1.BrokerSpec{
			ChannelTemplate: &v1alpha1.ChannelSpec{
				Provisioner: makeChannelProvisioner(),
			},
		},
	}
}

func makeReadyBroker() *v1alpha1.Broker {
	b := makeBroker()
	b.Status = *v1alpha1.TestHelper.ReadyBrokerStatus()
	return b
}

func makeChannelProvisioner() *corev1.ObjectReference {
	return &corev1.ObjectReference{
		APIVersion: "eventing.knative.dev/v1alpha1",
		Kind:       "ClusterChannelProvisioner",
		Name:       "my-provisioner",
	}
}

func newChannel(name string, labels map[string]string) *v1alpha1.Channel {
	return &v1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      name,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				getOwnerReference(),
			},
		},
		Spec: v1alpha1.ChannelSpec{
			Provisioner: makeChannelProvisioner(),
		},
		Status: v1alpha1.ChannelStatus{
			Address: duckv1alpha1.Addressable{
				Hostname: "any-non-empty-string",
			},
		},
	}
}

func makeTriggerChannel() *v1alpha1.Channel {
	labels := map[string]string{
		"eventing.knative.dev/broker":           brokerName,
		"eventing.knative.dev/brokerEverything": "true",
	}
	return newChannel(fmt.Sprintf("%s-broker", brokerName), labels)
}

func makeIngressChannel() *v1alpha1.Channel {
	labels := map[string]string{
		"eventing.knative.dev/broker":        brokerName,
		"eventing.knative.dev/brokerIngress": "true",
	}
	return newChannel(fmt.Sprintf("%s-broker-ingress", brokerName), labels)
}

func makeSubscriberServiceAsUnstructured() *unstructured.Unstructured {
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

func makeBrokerFilterService() *corev1.Service {
	return brokerresources.MakeFilterService(makeBroker())
}

func makeServiceURI() *url.URL {
	return &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s.%s.svc.%s", makeBrokerFilterService().Name, testNS, utils.GetClusterDomainName()),
		Path:   fmt.Sprintf("/triggers/%s/%s/%s", testNS, triggerName, triggerUID),
	}
}

func makeIngressSubscription() *v1alpha1.Subscription {
	return resources.NewSubscription(makeTrigger(), makeTriggerChannel(), makeIngressChannel(), makeServiceURI())
}

// Just so we can test subscription updates
func makeDifferentReadySubscription() *v1alpha1.Subscription {
	uri := "http://example.com/differenturi"
	s := makeIngressSubscription()
	s.Spec.Subscriber.URI = &uri
	s.Status = *v1alpha1.TestHelper.ReadySubscriptionStatus()
	return s
}

func makeReadySubscription() *v1alpha1.Subscription {
	s := makeIngressSubscription()
	s.Status = *v1alpha1.TestHelper.ReadySubscriptionStatus()
	return s
}

func makeNotReadySubscription() *v1alpha1.Subscription {
	s := makeIngressSubscription()
	s.Status = *v1alpha1.TestHelper.NotReadySubscriptionStatus()
	return s
}

func getOwnerReference() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         v1alpha1.SchemeGroupVersion.String(),
		Kind:               "Broker",
		Name:               brokerName,
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
	}
}
