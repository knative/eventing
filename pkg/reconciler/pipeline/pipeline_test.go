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

package pipeline

import (
	"fmt"
	//	"net/url"
	"testing"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/apis/messaging/v1alpha1"
	fakeclientset "github.com/knative/eventing/pkg/client/clientset/versioned/fake"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"github.com/knative/eventing/pkg/reconciler"
	//	"github.com/knative/pkg/kmeta"
	"github.com/knative/eventing/pkg/reconciler/pipeline/resources"
	reconciletesting "github.com/knative/eventing/pkg/reconciler/testing"
	//	"github.com/knative/eventing/pkg/utils"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	logtesting "github.com/knative/pkg/logging/testing"
	. "github.com/knative/pkg/reconciler/testing"
	//	"github.com/knative/pkg/tracker"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	//	kubeinformers "k8s.io/client-go/informers"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
)

const (
	testNS       = "test-namespace"
	pipelineName = "test-pipeline"
	pipelineUID  = "test-pipeline-uid"
	brokerName   = "test-broker"

	channelServiceAddress = "test-pipeline-kn-channel.test-namespace.svc.cluster.local"

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
	//	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
	eventingInformerFactory := informers.NewSharedInformerFactory(eventingClient, 0)

	// Messaging
	pipelineInformer := eventingInformerFactory.Messaging().V1alpha1().Pipelines()

	// Eventing
	channelInformer := eventingInformerFactory.Eventing().V1alpha1().Channels()
	subscriptionInformer := eventingInformerFactory.Eventing().V1alpha1().Subscriptions()

	// Kube
	//	serviceInformer := kubeInformerFactory.Core().V1().Services()
	//endpointsInformer := kubeInformerFactory.Core().V1().Endpoints()
	//	deploymentInformer := kubeInformerFactory.Apps().V1().Deployments()

	c := NewController(
		reconciler.Options{
			KubeClientSet:     kubeClient,
			EventingClientSet: eventingClient,
			Logger:            logtesting.TestLogger(t),
		},
		pipelineInformer,
		channelInformer,
		subscriptionInformer)

	if c == nil {
		t.Fatalf("Failed to create with NewController")
	}
}

func createChannel(pipelineName string, stepNumber int) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "messaging.knative.dev/v1alpha1",
			"kind":       "inmemorychannel",
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"namespace":         testNS,
				"name":              resources.PipelineChannelName(pipelineName, stepNumber),
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "messaging.knative.dev/v1alpha1",
						"blockOwnerDeletion": true,
						"controller":         true,
						"kind":               "InMemoryChannel",
						"name":               pipelineName,
						"uid":                "",
					},
				},
			},
			"spec": map[string]interface{}{},
			//			"spec": map[string]interface{}{
			//				"something": "foo",
			//			},
		},
	}

}

func createSubscriber(stepNumber int) eventingv1alpha1.SubscriberSpec {
	uriString := fmt.Sprintf("http://example.com/%d", stepNumber)
	return eventingv1alpha1.SubscriberSpec{
		URI: &uriString,
	}
}

func TestAllCases(t *testing.T) {
	pKey := testNS + "/" + pipelineName
	imc := v1alpha1.ChannelTemplateSpec{
		metav1.TypeMeta{
			APIVersion: "messaging.knative.dev/v1alpha1",
			Kind:       "inmemorychannel",
		},
		runtime.RawExtension{Raw: []byte("{}")},
	}

	/*
		imcWithSpec := v1alpha1.ChannelTemplateSpec{
			metav1.TypeMeta{
				APIVersion: "messaging.knative.dev/v1alpha1",
				Kind:       "inmemorychannel",
			},
			metav1.ObjectMeta{},
			runtime.RawExtension{Raw: []byte("{}")},
		}
	*/

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
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewPipeline(pipelineName, testNS,
					reconciletesting.WithInitPipelineConditions,
					reconciletesting.WithPipelineDeleted)},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Pipeline reconciled"),
			},
		}, {
			Name: "singlestep",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewPipeline(pipelineName, testNS,
					reconciletesting.WithInitPipelineConditions,
					reconciletesting.WithPipelineChannelTemplateSpec(imc),
					reconciletesting.WithPipelineSteps([]eventingv1alpha1.SubscriberSpec{createSubscriber(0)}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Pipeline reconciled"),
			},
			WantCreates: []runtime.Object{
				createChannel(pipelineName, 0),
				resources.NewSubscription(0, reconciletesting.NewPipeline(pipelineName, testNS, reconciletesting.WithPipelineChannelTemplateSpec(imc), reconciletesting.WithPipelineSteps([]eventingv1alpha1.SubscriberSpec{createSubscriber(0)}))),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewPipeline(pipelineName, testNS,
					reconciletesting.WithInitPipelineConditions,
					reconciletesting.WithPipelineChannelTemplateSpec(imc),
					reconciletesting.WithPipelineSteps([]eventingv1alpha1.SubscriberSpec{createSubscriber(0)}),
					reconciletesting.WithPipelineChannelStatuses([]v1alpha1.PipelineChannelStatus{
						v1alpha1.PipelineChannelStatus{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1alpha1",
								Kind:       "inmemorychannel",
								Name:       resources.PipelineChannelName(pipelineName, 0),
							},
						},
					}),
					reconciletesting.WithPipelineSubscriptionStatuses([]v1alpha1.PipelineSubscriptionStatus{
						v1alpha1.PipelineSubscriptionStatus{
							Subscription: corev1.ObjectReference{
								APIVersion: "eventing.knative.dev/v1alpha1",
								Kind:       "Subscription",
								Name:       resources.PipelineSubscriptionName(pipelineName, 0),
							},
						},
					})),
			}},
		}, {
			Name: "threestep",
			Key:  pKey,
			Objects: []runtime.Object{
				reconciletesting.NewPipeline(pipelineName, testNS,
					reconciletesting.WithInitPipelineConditions,
					reconciletesting.WithPipelineChannelTemplateSpec(imc),
					reconciletesting.WithPipelineSteps([]eventingv1alpha1.SubscriberSpec{
						createSubscriber(0),
						createSubscriber(1),
						createSubscriber(2)}))},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "Reconciled", "Pipeline reconciled"),
			},
			WantCreates: []runtime.Object{
				createChannel(pipelineName, 0),
				createChannel(pipelineName, 1),
				createChannel(pipelineName, 2),
				resources.NewSubscription(0, reconciletesting.NewPipeline(pipelineName, testNS, reconciletesting.WithPipelineChannelTemplateSpec(imc), reconciletesting.WithPipelineSteps([]eventingv1alpha1.SubscriberSpec{createSubscriber(0), createSubscriber(1), createSubscriber(2)}))),
				resources.NewSubscription(1, reconciletesting.NewPipeline(pipelineName, testNS, reconciletesting.WithPipelineChannelTemplateSpec(imc), reconciletesting.WithPipelineSteps([]eventingv1alpha1.SubscriberSpec{createSubscriber(0), createSubscriber(1), createSubscriber(2)}))),
				resources.NewSubscription(2, reconciletesting.NewPipeline(pipelineName, testNS, reconciletesting.WithPipelineChannelTemplateSpec(imc), reconciletesting.WithPipelineSteps([]eventingv1alpha1.SubscriberSpec{createSubscriber(0), createSubscriber(1), createSubscriber(2)})))},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewPipeline(pipelineName, testNS,
					reconciletesting.WithInitPipelineConditions,
					reconciletesting.WithPipelineChannelTemplateSpec(imc),
					reconciletesting.WithPipelineSteps([]eventingv1alpha1.SubscriberSpec{
						createSubscriber(0),
						createSubscriber(1),
						createSubscriber(2),
					}),
					reconciletesting.WithPipelineChannelStatuses([]v1alpha1.PipelineChannelStatus{
						v1alpha1.PipelineChannelStatus{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1alpha1",
								Kind:       "inmemorychannel",
								Name:       resources.PipelineChannelName(pipelineName, 0),
							},
						},
						v1alpha1.PipelineChannelStatus{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1alpha1",
								Kind:       "inmemorychannel",
								Name:       resources.PipelineChannelName(pipelineName, 1),
							},
						},
						v1alpha1.PipelineChannelStatus{
							Channel: corev1.ObjectReference{
								APIVersion: "messaging.knative.dev/v1alpha1",
								Kind:       "inmemorychannel",
								Name:       resources.PipelineChannelName(pipelineName, 2),
							},
						},
					}),
					reconciletesting.WithPipelineSubscriptionStatuses([]v1alpha1.PipelineSubscriptionStatus{
						v1alpha1.PipelineSubscriptionStatus{
							Subscription: corev1.ObjectReference{
								APIVersion: "eventing.knative.dev/v1alpha1",
								Kind:       "Subscription",
								Name:       resources.PipelineSubscriptionName(pipelineName, 0),
							},
						},
						v1alpha1.PipelineSubscriptionStatus{
							Subscription: corev1.ObjectReference{
								APIVersion: "eventing.knative.dev/v1alpha1",
								Kind:       "Subscription",
								Name:       resources.PipelineSubscriptionName(pipelineName, 1),
							},
						},
						v1alpha1.PipelineSubscriptionStatus{
							Subscription: corev1.ObjectReference{
								APIVersion: "eventing.knative.dev/v1alpha1",
								Kind:       "Subscription",
								Name:       resources.PipelineSubscriptionName(pipelineName, 2),
							},
						},
					})),
			}},
		},
	}

	defer logtesting.ClearAll()

	table.Test(t, reconciletesting.MakeFactory(func(listers *reconciletesting.Listers, opt reconciler.Options) controller.Reconciler {
		return &Reconciler{
			Base:               reconciler.NewBase(opt, controllerAgentName),
			pipelineLister:     listers.GetPipelineLister(),
			channelLister:      listers.GetChannelLister(),
			subscriptionLister: listers.GetSubscriptionLister(),
		}
	},
		false,
	))
}
