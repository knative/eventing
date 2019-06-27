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

	"github.com/Shopify/sarama"
	. "github.com/knative/eventing/contrib/kafka/pkg/utils"
	"github.com/knative/eventing/pkg/utils"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/kmeta"

	"github.com/knative/eventing/contrib/kafka/pkg/apis/messaging/v1alpha1"
	fakeclientset "github.com/knative/eventing/contrib/kafka/pkg/client/clientset/versioned/fake"
	informers "github.com/knative/eventing/contrib/kafka/pkg/client/informers/externalversions"
	"github.com/knative/eventing/contrib/kafka/pkg/reconciler"
	"github.com/knative/eventing/contrib/kafka/pkg/reconciler/controller/resources"
	reconciletesting "github.com/knative/eventing/contrib/kafka/pkg/reconciler/testing"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	systemNS                 = "knative-eventing"
	testNS                   = "test-namespace"
	kcName                   = "test-kc"
	dispatcherDeploymentName = "test-deployment"
	dispatcherServiceName    = "test-service"
	channelServiceAddress    = "test-kc-kn-channel.test-namespace.svc.cluster.local"
	brokerName               = "test-broker"
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
	ncInformer := messagingInformerFactory.Messaging().V1alpha1().KafkaChannels()

	// Kube
	serviceInformer := kubeInformerFactory.Core().V1().Services()
	endpointsInformer := kubeInformerFactory.Core().V1().Endpoints()
	deploymentInformer := kubeInformerFactory.Apps().V1().Deployments()

	kafkaConfig := &KafkaConfig{}

	c := NewController(
		reconciler.Options{
			KubeClientSet:  kubeClient,
			KafkaClientSet: messagingClient,
			Logger:         logtesting.TestLogger(t),
		},
		kafkaConfig,
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
	kcKey := testNS + "/" + kcName
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
			Key:  kcKey,
			Objects: []runtime.Object{
				reconciletesting.NewKafkaChannel(kcName, testNS,
					reconciletesting.WithInitKafkaChannelConditions,
					reconciletesting.WithKafkaChannelDeleted)},
			WantErr: false,
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewKafkaChannel(kcName, testNS,
					reconciletesting.WithInitKafkaChannelConditions,
					reconciletesting.WithKafkaChannelDeleted),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, channelReconciled, "KafkaChannel reconciled"),
			},
		}, {
			Name: "deployment does not exist, and patching finalizers",
			Key:  kcKey,
			Objects: []runtime.Object{
				reconciletesting.NewKafkaChannel(kcName, testNS,
					reconciletesting.WithInitKafkaChannelConditions,
					reconciletesting.WithKafkaChannelTopicReady()),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewKafkaChannel(kcName, testNS,
					reconciletesting.WithInitKafkaChannelConditions,
					reconciletesting.WithKafkaChannelTopicReady(),
					reconciletesting.WithKafkaChannelDeploymentNotReady("DispatcherDeploymentDoesNotExist", "Dispatcher Deployment does not exist")),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, kcName),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, channelReconcileFailed, "KafkaChannel reconciliation failed: deployment.apps \"test-deployment\" not found"),
			},
		}, {
			Name: "Service does not exist",
			Key:  kcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				reconciletesting.NewKafkaChannel(kcName, testNS,
					reconciletesting.WithKafkaFinalizer(finalizerName)),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewKafkaChannel(kcName, testNS,
					reconciletesting.WithInitKafkaChannelConditions,
					reconciletesting.WithKafkaFinalizer(finalizerName),
					reconciletesting.WithKafkaChannelTopicReady(),
					reconciletesting.WithKafkaChannelDeploymentReady(),
					reconciletesting.WithKafkaChannelServicetNotReady("DispatcherServiceDoesNotExist", "Dispatcher Service does not exist")),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, channelReconcileFailed, "KafkaChannel reconciliation failed: service \"test-service\" not found"),
			},
		}, {
			Name: "Endpoints does not exist",
			Key:  kcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				reconciletesting.NewKafkaChannel(kcName, testNS,
					reconciletesting.WithKafkaFinalizer(finalizerName)),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewKafkaChannel(kcName, testNS,
					reconciletesting.WithInitKafkaChannelConditions,
					reconciletesting.WithKafkaFinalizer(finalizerName),
					reconciletesting.WithKafkaChannelTopicReady(),
					reconciletesting.WithKafkaChannelDeploymentReady(),
					reconciletesting.WithKafkaChannelServiceReady(),
					reconciletesting.WithKafkaChannelEndpointsNotReady("DispatcherEndpointsDoesNotExist", "Dispatcher Endpoints does not exist"),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, channelReconcileFailed, "KafkaChannel reconciliation failed: endpoints \"test-service\" not found"),
			},
		}, {
			Name: "Endpoints not ready",
			Key:  kcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeEmptyEndpoints(),
				reconciletesting.NewKafkaChannel(kcName, testNS,
					reconciletesting.WithKafkaFinalizer(finalizerName)),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewKafkaChannel(kcName, testNS,
					reconciletesting.WithInitKafkaChannelConditions,
					reconciletesting.WithKafkaFinalizer(finalizerName),
					reconciletesting.WithKafkaChannelTopicReady(),
					reconciletesting.WithKafkaChannelDeploymentReady(),
					reconciletesting.WithKafkaChannelServiceReady(),
					reconciletesting.WithKafkaChannelEndpointsNotReady("DispatcherEndpointsNotReady", "There are no endpoints ready for Dispatcher service"),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, channelReconcileFailed, "KafkaChannel reconciliation failed: there are no endpoints ready for Dispatcher service test-service"),
			},
		}, {
			Name: "Works, creates new channel",
			Key:  kcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				reconciletesting.NewKafkaChannel(kcName, testNS,
					reconciletesting.WithKafkaFinalizer(finalizerName)),
			},
			WantErr: false,
			WantCreates: []runtime.Object{
				makeChannelService(reconciletesting.NewKafkaChannel(kcName, testNS)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewKafkaChannel(kcName, testNS,
					reconciletesting.WithInitKafkaChannelConditions,
					reconciletesting.WithKafkaFinalizer(finalizerName),
					reconciletesting.WithKafkaChannelTopicReady(),
					reconciletesting.WithKafkaChannelDeploymentReady(),
					reconciletesting.WithKafkaChannelServiceReady(),
					reconciletesting.WithKafkaChannelEndpointsReady(),
					reconciletesting.WithKafkaChannelChannelServiceReady(),
					reconciletesting.WithKafkaChannelAddress(channelServiceAddress),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, channelReconciled, "KafkaChannel reconciled"),
			},
		}, {
			Name: "Works, channel exists",
			Key:  kcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				reconciletesting.NewKafkaChannel(kcName, testNS,
					reconciletesting.WithKafkaFinalizer(finalizerName)),
				makeChannelService(reconciletesting.NewKafkaChannel(kcName, testNS)),
			},
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewKafkaChannel(kcName, testNS,
					reconciletesting.WithInitKafkaChannelConditions,
					reconciletesting.WithKafkaFinalizer(finalizerName),
					reconciletesting.WithKafkaChannelTopicReady(),
					reconciletesting.WithKafkaChannelDeploymentReady(),
					reconciletesting.WithKafkaChannelServiceReady(),
					reconciletesting.WithKafkaChannelEndpointsReady(),
					reconciletesting.WithKafkaChannelChannelServiceReady(),
					reconciletesting.WithKafkaChannelAddress(channelServiceAddress),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, channelReconciled, "KafkaChannel reconciled"),
			},
		}, {
			Name: "channel exists, not owned by us",
			Key:  kcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				reconciletesting.NewKafkaChannel(kcName, testNS,
					reconciletesting.WithKafkaFinalizer(finalizerName)),
				makeChannelServiceNotOwnedByUs(reconciletesting.NewKafkaChannel(kcName, testNS)),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewKafkaChannel(kcName, testNS,
					reconciletesting.WithInitKafkaChannelConditions,
					reconciletesting.WithKafkaFinalizer(finalizerName),
					reconciletesting.WithKafkaChannelTopicReady(),
					reconciletesting.WithKafkaChannelDeploymentReady(),
					reconciletesting.WithKafkaChannelServiceReady(),
					reconciletesting.WithKafkaChannelEndpointsReady(),
					reconciletesting.WithKafkaChannelChannelServicetNotReady("ChannelServiceFailed", "Channel Service failed: kafkachannel: test-namespace/test-kc does not own Service: \"test-kc-kn-channel\""),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, channelReconcileFailed, "KafkaChannel reconciliation failed: kafkachannel: test-namespace/test-kc does not own Service: \"test-kc-kn-channel\""),
			},
		}, {
			Name: "channel does not exist, fails to create",
			Key:  kcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				reconciletesting.NewKafkaChannel(kcName, testNS,
					reconciletesting.WithKafkaFinalizer(finalizerName)),
			},
			WantErr: true,
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "Services"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconciletesting.NewKafkaChannel(kcName, testNS,
					reconciletesting.WithInitKafkaChannelConditions,
					reconciletesting.WithKafkaFinalizer(finalizerName),
					reconciletesting.WithKafkaChannelTopicReady(),
					reconciletesting.WithKafkaChannelDeploymentReady(),
					reconciletesting.WithKafkaChannelServiceReady(),
					reconciletesting.WithKafkaChannelEndpointsReady(),
					reconciletesting.WithKafkaChannelChannelServicetNotReady("ChannelServiceFailed", "Channel Service failed: inducing failure for create services"),
				),
			}},
			WantCreates: []runtime.Object{
				makeChannelService(reconciletesting.NewKafkaChannel(kcName, testNS)),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, channelReconcileFailed, "KafkaChannel reconciliation failed: inducing failure for create services"),
			},
			// TODO add UTs for topic creation and deletion.
		},
	}
	defer logtesting.ClearAll()

	table.Test(t, reconciletesting.MakeFactory(func(listers *reconciletesting.Listers, opt reconciler.Options) controller.Reconciler {
		return &Reconciler{
			Base:                     reconciler.NewBase(opt, controllerAgentName),
			dispatcherNamespace:      testNS,
			dispatcherDeploymentName: dispatcherDeploymentName,
			dispatcherServiceName:    dispatcherServiceName,
			kafkaConfig: &KafkaConfig{
				Brokers: []string{brokerName},
			},
			kafkachannelLister: listers.GetKafkaChannelLister(),
			// TODO fix
			kafkachannelInformer: nil,
			deploymentLister:     listers.GetDeploymentLister(),
			serviceLister:        listers.GetServiceLister(),
			endpointsLister:      listers.GetEndpointsLister(),
			kafkaClusterAdmin:    &mockClusterAdmin{},
		}
	}))
}

type mockClusterAdmin struct {
	mockCreateTopicFunc func(topic string, detail *sarama.TopicDetail, validateOnly bool) error
	mockDeleteTopicFunc func(topic string) error
}

func (ca *mockClusterAdmin) CreateTopic(topic string, detail *sarama.TopicDetail, validateOnly bool) error {
	if ca.mockCreateTopicFunc != nil {
		return ca.mockCreateTopicFunc(topic, detail, validateOnly)
	}
	return nil
}

func (ca *mockClusterAdmin) Close() error {
	return nil
}

func (ca *mockClusterAdmin) DeleteTopic(topic string) error {
	if ca.mockDeleteTopicFunc != nil {
		return ca.mockDeleteTopicFunc(topic)
	}
	return nil
}

func (ca *mockClusterAdmin) CreatePartitions(topic string, count int32, assignment [][]int32, validateOnly bool) error {
	return nil
}

func (ca *mockClusterAdmin) DeleteRecords(topic string, partitionOffsets map[int32]int64) error {
	return nil
}

func (ca *mockClusterAdmin) DescribeConfig(resource sarama.ConfigResource) ([]sarama.ConfigEntry, error) {
	return nil, nil
}

func (ca *mockClusterAdmin) AlterConfig(resourceType sarama.ConfigResourceType, name string, entries map[string]*string, validateOnly bool) error {
	return nil
}

func (ca *mockClusterAdmin) CreateACL(resource sarama.Resource, acl sarama.Acl) error {
	return nil
}

func (ca *mockClusterAdmin) ListAcls(filter sarama.AclFilter) ([]sarama.ResourceAcls, error) {
	return nil, nil
}

func (ca *mockClusterAdmin) DeleteACL(filter sarama.AclFilter, validateOnly bool) ([]sarama.MatchingAcl, error) {
	return nil, nil
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

func makeChannelService(nc *v1alpha1.KafkaChannel) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      fmt.Sprintf("%s-kn-channel", kcName),
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

func makeChannelServiceNotOwnedByUs(nc *v1alpha1.KafkaChannel) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      fmt.Sprintf("%s-kn-channel", kcName),
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

func patchFinalizers(namespace, name string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	patch := `{"metadata":{"finalizers":["` + finalizerName + `"],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}
