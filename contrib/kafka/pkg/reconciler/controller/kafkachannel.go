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
	"encoding/json"
	"fmt"
	"github.com/knative/pkg/apis"
	"reflect"
	"time"

	"github.com/knative/eventing/contrib/kafka/pkg/utils"
	"github.com/knative/eventing/pkg/reconciler/names"

	"github.com/Shopify/sarama"
	"github.com/knative/eventing/contrib/kafka/pkg/apis/messaging/v1alpha1"
	clientset "github.com/knative/eventing/contrib/kafka/pkg/client/clientset/versioned"
	messaginginformers "github.com/knative/eventing/contrib/kafka/pkg/client/informers/externalversions/messaging/v1alpha1"
	listers "github.com/knative/eventing/contrib/kafka/pkg/client/listers/messaging/v1alpha1"
	"github.com/knative/eventing/contrib/kafka/pkg/reconciler/controller/resources"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/pkg/controller"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	// ReconcilerName is the name of the reconciler.
	ReconcilerName = "KafkaChannels"

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "kafka-ch-controller"

	finalizerName = controllerAgentName

	// Name of the corev1.Events emitted from the reconciliation process.
	channelReconciled         = "ChannelReconciled"
	channelReconcileFailed    = "ChannelReconcileFailed"
	channelUpdateStatusFailed = "ChannelUpdateStatusFailed"
)

// Reconciler reconciles Kafka Channels.
type Reconciler struct {
	*reconciler.Base

	dispatcherNamespace      string
	dispatcherDeploymentName string
	dispatcherServiceName    string

	kafkaConfig *utils.KafkaConfig

	// Using a shared kafkaClusterAdmin does not work currently because of an issue with
	// Shopify/sarama, see https://github.com/Shopify/sarama/issues/1162.
	kafkaClusterAdmin sarama.ClusterAdmin

	eventingClientSet    clientset.Interface
	kafkachannelLister   listers.KafkaChannelLister
	kafkachannelInformer cache.SharedIndexInformer
	deploymentLister     appsv1listers.DeploymentLister
	serviceLister        corev1listers.ServiceLister
	endpointsLister      corev1listers.EndpointsLister
	impl                 *controller.Impl
}

var (
	deploymentGVK = appsv1.SchemeGroupVersion.WithKind("Deployment")
	serviceGVK    = corev1.SchemeGroupVersion.WithKind("Service")
)

// Check that our Reconciler implements controller.Reconciler.
var _ controller.Reconciler = (*Reconciler)(nil)

// Check that our Reconciler implements cache.ResourceEventHandler
var _ cache.ResourceEventHandler = (*Reconciler)(nil)

// NewController initializes the controller and is called by the generated code.
// Registers event handlers to enqueue events.
func NewController(
	opt reconciler.Options,
	eventingClientSet clientset.Interface,
	kafkaConfig *utils.KafkaConfig,
	dispatcherNamespace string,
	dispatcherDeploymentName string,
	dispatcherServiceName string,
	kafkachannelInformer messaginginformers.KafkaChannelInformer,
	deploymentInformer appsv1informers.DeploymentInformer,
	serviceInformer corev1informers.ServiceInformer,
	endpointsInformer corev1informers.EndpointsInformer,
) *controller.Impl {

	r := &Reconciler{
		Base:                     reconciler.NewBase(opt, controllerAgentName),
		dispatcherNamespace:      dispatcherNamespace,
		dispatcherDeploymentName: dispatcherDeploymentName,
		dispatcherServiceName:    dispatcherServiceName,
		kafkaConfig:              kafkaConfig,
		eventingClientSet:        eventingClientSet,
		kafkachannelLister:       kafkachannelInformer.Lister(),
		kafkachannelInformer:     kafkachannelInformer.Informer(),
		deploymentLister:         deploymentInformer.Lister(),
		serviceLister:            serviceInformer.Lister(),
		endpointsLister:          endpointsInformer.Lister(),
	}
	r.impl = controller.NewImpl(r, r.Logger, ReconcilerName)

	r.Logger.Info("Setting up event handlers")
	kafkachannelInformer.Informer().AddEventHandler(controller.HandleAll(r.impl.Enqueue))

	// Set up watches for dispatcher resources we care about, since any changes to these
	// resources will affect our Channels. So, set up a watch here, that will cause
	// a global Resync for all the channels to take stock of their health when these change.
	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithNameAndNamespace(dispatcherNamespace, dispatcherDeploymentName),
		Handler:    r,
	})
	serviceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithNameAndNamespace(dispatcherNamespace, dispatcherServiceName),
		Handler:    r,
	})
	endpointsInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithNameAndNamespace(dispatcherNamespace, dispatcherServiceName),
		Handler:    r,
	})
	return r.impl
}

// cache.ResourceEventHandler implementation.
// These 3 functions just cause a Global Resync of the channels, because any changes here
// should be reflected onto the channels.
func (r *Reconciler) OnAdd(obj interface{}) {
	r.impl.GlobalResync(r.kafkachannelInformer)
}

func (r *Reconciler) OnUpdate(old, new interface{}) {
	r.impl.GlobalResync(r.kafkachannelInformer)
}

func (r *Reconciler) OnDelete(obj interface{}) {
	r.impl.GlobalResync(r.kafkachannelInformer)
}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the KafkaChannel resource
// with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key")
		return nil
	}

	// Get the KafkaChannel resource with this namespace/name.
	original, err := r.kafkachannelLister.KafkaChannels(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("KafkaChannel key in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy.
	channel := original.DeepCopy()

	// Reconcile this copy of the KafkaChannel and then write back any status updates regardless of
	// whether the reconcile error out.
	reconcileErr := r.reconcile(ctx, channel)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling KafkaChannel", zap.Error(reconcileErr))
		r.Recorder.Eventf(channel, corev1.EventTypeWarning, channelReconcileFailed, "KafkaChannel reconciliation failed: %v", reconcileErr)
	} else {
		logging.FromContext(ctx).Debug("KafkaChannel reconciled")
		r.Recorder.Event(channel, corev1.EventTypeNormal, channelReconciled, "KafkaChannel reconciled")
	}

	if _, updateStatusErr := r.updateStatus(ctx, channel); updateStatusErr != nil {
		logging.FromContext(ctx).Error("Failed to update KafkaChannel status", zap.Error(updateStatusErr))
		r.Recorder.Eventf(channel, corev1.EventTypeWarning, channelUpdateStatusFailed, "Failed to update KafkaChannel's status: %v", updateStatusErr)
		return updateStatusErr
	}

	// Requeue if the resource is not ready
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, kc *v1alpha1.KafkaChannel) error {
	kc.Status.InitializeConditions()

	logger := logging.FromContext(ctx)
	// Verify channel is valid.
	if err := kc.Validate(ctx); err != nil {
		logger.Error("Invalid kafka channel", zap.String("channel", kc.Name), zap.Error(err))
		return err
	}

	kafkaClusterAdmin, err := r.createClient(ctx, kc)
	if err != nil {
		logger.Error("Unable to build kafka admin client", zap.String("channel", kc.Name), zap.Error(err))
		return err
	}

	// See if the channel has been deleted.
	if kc.DeletionTimestamp != nil {
		if err := r.deleteTopic(ctx, kc, kafkaClusterAdmin); err != nil {
			return err
		}
		removeFinalizer(kc)
		_, err := r.eventingClientSet.MessagingV1alpha1().KafkaChannels(kc.Namespace).Update(kc)
		return err
	}

	// If we are adding the finalizer for the first time, then ensure that finalizer is persisted
	// before manipulating Kafka.
	if err := r.ensureFinalizer(kc); err != nil {
		return err
	}

	// We reconcile the status of the Channel by looking at:
	// 1. Kafka topic used by the channel.
	// 2. Dispatcher Deployment for it's readiness.
	// 3. Dispatcher k8s Service for it's existence.
	// 4. Dispatcher endpoints to ensure that there's something backing the Service.
	// 5. K8s service representing the channel that will use ExternalName to point to the Dispatcher k8s service.

	if err := r.createTopic(ctx, kc, kafkaClusterAdmin); err != nil {
		kc.Status.MarkTopicFailed("TopicCreateFailed", "error while creating topic: %s", err)
		return err
	}
	kc.Status.MarkTopicTrue()

	// Get the Dispatcher Deployment and propagate the status to the Channel
	d, err := r.deploymentLister.Deployments(r.dispatcherNamespace).Get(r.dispatcherDeploymentName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			kc.Status.MarkDispatcherFailed("DispatcherDeploymentDoesNotExist", "Dispatcher Deployment does not exist")
		} else {
			logger.Error("Unable to get the dispatcher Deployment", zap.Error(err))
			kc.Status.MarkDispatcherFailed("DispatcherDeploymentGetFailed", "Failed to get dispatcher Deployment")
		}
		return err
	}
	kc.Status.PropagateDispatcherStatus(&d.Status)

	// Get the Dispatcher Service and propagate the status to the Channel in case it does not exist.
	// We don't do anything with the service because it's status contains nothing useful, so just do
	// an existence check. Then below we check the endpoints targeting it.
	_, err = r.serviceLister.Services(r.dispatcherNamespace).Get(r.dispatcherServiceName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			kc.Status.MarkServiceFailed("DispatcherServiceDoesNotExist", "Dispatcher Service does not exist")
		} else {
			logger.Error("Unable to get the dispatcher service", zap.Error(err))
			kc.Status.MarkServiceFailed("DispatcherServiceGetFailed", "Failed to get dispatcher service")
		}
		return err
	}
	kc.Status.MarkServiceTrue()

	// Get the Dispatcher Service Endpoints and propagate the status to the Channel
	// endpoints has the same name as the service, so not a bug.
	e, err := r.endpointsLister.Endpoints(r.dispatcherNamespace).Get(r.dispatcherServiceName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			kc.Status.MarkEndpointsFailed("DispatcherEndpointsDoesNotExist", "Dispatcher Endpoints does not exist")
		} else {
			logger.Error("Unable to get the dispatcher endpoints", zap.Error(err))
			kc.Status.MarkEndpointsFailed("DispatcherEndpointsGetFailed", "Failed to get dispatcher endpoints")
		}
		return err
	}

	if len(e.Subsets) == 0 {
		logger.Error("No endpoints found for Dispatcher service", zap.Error(err))
		kc.Status.MarkEndpointsFailed("DispatcherEndpointsNotReady", "There are no endpoints ready for Dispatcher service")
		return fmt.Errorf("there are no endpoints ready for Dispatcher service %s", r.dispatcherServiceName)
	}
	kc.Status.MarkEndpointsTrue()

	// Reconcile the k8s service representing the actual Channel. It points to the Dispatcher service via ExternalName
	svc, err := r.reconcileChannelService(ctx, kc)
	if err != nil {
		kc.Status.MarkChannelServiceFailed("ChannelServiceFailed", fmt.Sprintf("Channel Service failed: %s", err))
		return err
	}
	kc.Status.MarkChannelServiceTrue()
	kc.Status.SetAddress(&apis.URL{
		Scheme: "http",
		Host:   names.ServiceHostName(svc.Name, svc.Namespace),
	})

	// close the connection
	err = kafkaClusterAdmin.Close()
	if err != nil {
		logger.Error("Error closing the connection", zap.Error(err))
		return err
	}

	// Ok, so now the Dispatcher Deployment & Service have been created, we're golden since the
	// dispatcher watches the Channel and where it needs to dispatch events to.
	return nil
}

func (r *Reconciler) reconcileChannelService(ctx context.Context, channel *v1alpha1.KafkaChannel) (*corev1.Service, error) {
	logger := logging.FromContext(ctx)
	// Get the  Service and propagate the status to the Channel in case it does not exist.
	// We don't do anything with the service because it's status contains nothing useful, so just do
	// an existence check. Then below we check the endpoints targeting it.
	// We may change this name later, so we have to ensure we use proper addressable when resolving these.
	svc, err := r.serviceLister.Services(channel.Namespace).Get(resources.MakeChannelServiceName(channel.Name))
	if err != nil {
		if apierrs.IsNotFound(err) {
			svc, err = resources.MakeK8sService(channel, resources.ExternalService(r.dispatcherNamespace, r.dispatcherServiceName))
			if err != nil {
				logger.Error("Failed to create the channel service object", zap.Error(err))
				return nil, err
			}
			svc, err = r.KubeClientSet.CoreV1().Services(channel.Namespace).Create(svc)
			if err != nil {
				logger.Error("Failed to create the channel service", zap.Error(err))
				return nil, err
			}
			return svc, nil
		} else {
			logger.Error("Unable to get the channel service", zap.Error(err))
		}
		return nil, err
	}
	// Check to make sure that the KafkaChannel owns this service and if not, complain.
	if !metav1.IsControlledBy(svc, channel) {
		return nil, fmt.Errorf("kafkachannel: %s/%s does not own Service: %q", channel.Namespace, channel.Name, svc.Name)
	}
	return svc, nil
}

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.KafkaChannel) (*v1alpha1.KafkaChannel, error) {
	kc, err := r.kafkachannelLister.KafkaChannels(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}

	if reflect.DeepEqual(kc.Status, desired.Status) {
		return kc, nil
	}

	becomesReady := desired.Status.IsReady() && !kc.Status.IsReady()

	// Don't modify the informers copy.
	existing := kc.DeepCopy()
	existing.Status = desired.Status

	new, err := r.eventingClientSet.MessagingV1alpha1().KafkaChannels(desired.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(new.ObjectMeta.CreationTimestamp.Time)
		r.Logger.Infof("KafkaChannel %q became ready after %v", kc.Name, duration)
		if err := r.StatsReporter.ReportReady("Channel", kc.Namespace, kc.Name, duration); err != nil {
			r.Logger.Infof("Failed to record ready for KafkaChannel %q: %v", kc.Name, err)
		}
	}
	return new, err
}

func (r *Reconciler) createClient(ctx context.Context, kc *v1alpha1.KafkaChannel) (sarama.ClusterAdmin, error) {
	// We don't currently initialize r.kafkaClusterAdmin, hence we end up creating the cluster admin client every time.
	// This is because of an issue with Shopify/sarama. See https://github.com/Shopify/sarama/issues/1162.
	// Once the issue is fixed we should use a shared cluster admin client. Also, r.kafkaClusterAdmin is currently
	// used to pass a fake admin client in the tests.
	kafkaClusterAdmin := r.kafkaClusterAdmin
	if kafkaClusterAdmin == nil {
		var err error
		kafkaClusterAdmin, err = resources.MakeClient(controllerAgentName, r.kafkaConfig.Brokers)
		if err != nil {
			return nil, err
		}
	}
	return kafkaClusterAdmin, nil
}

func (r *Reconciler) createTopic(ctx context.Context, channel *v1alpha1.KafkaChannel, kafkaClusterAdmin sarama.ClusterAdmin) error {
	logger := logging.FromContext(ctx)

	topicName := utils.TopicName(utils.KafkaChannelSeparator, channel.Namespace, channel.Name)
	logger.Info("Creating topic on Kafka cluster", zap.String("topic", topicName))
	err := kafkaClusterAdmin.CreateTopic(topicName, &sarama.TopicDetail{
		ReplicationFactor: channel.Spec.ReplicationFactor,
		NumPartitions:     channel.Spec.NumPartitions,
	}, false)
	if err == sarama.ErrTopicAlreadyExists {
		return nil
	} else if err != nil {
		logger.Error("Error creating topic", zap.String("topic", topicName), zap.Error(err))
	} else {
		logger.Info("Successfully created topic", zap.String("topic", topicName))
	}
	return err
}

func (r *Reconciler) deleteTopic(ctx context.Context, channel *v1alpha1.KafkaChannel, kafkaClusterAdmin sarama.ClusterAdmin) error {
	logger := logging.FromContext(ctx)

	topicName := utils.TopicName(utils.KafkaChannelSeparator, channel.Namespace, channel.Name)
	logger.Info("Deleting topic on Kafka Cluster", zap.String("topic", topicName))
	err := kafkaClusterAdmin.DeleteTopic(topicName)
	if err == sarama.ErrUnknownTopicOrPartition {
		return nil
	} else if err != nil {
		logger.Error("Error deleting topic", zap.String("topic", topicName), zap.Error(err))
	} else {
		logger.Info("Successfully deleted topic", zap.String("topic", topicName))
	}
	return err
}

func (r *Reconciler) ensureFinalizer(channel *v1alpha1.KafkaChannel) error {
	finalizers := sets.NewString(channel.Finalizers...)
	if finalizers.Has(finalizerName) {
		return nil
	}

	mergePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers":      append(channel.Finalizers, finalizerName),
			"resourceVersion": channel.ResourceVersion,
		},
	}

	patch, err := json.Marshal(mergePatch)
	if err != nil {
		return err
	}

	_, err = r.eventingClientSet.MessagingV1alpha1().KafkaChannels(channel.Namespace).Patch(channel.Name, types.MergePatchType, patch)
	return err
}

func removeFinalizer(channel *v1alpha1.KafkaChannel) {
	finalizers := sets.NewString(channel.Finalizers...)
	finalizers.Delete(finalizerName)
	channel.Finalizers = finalizers.List()
}
