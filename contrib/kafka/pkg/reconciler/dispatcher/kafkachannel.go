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
	"reflect"

	"github.com/knative/eventing/contrib/kafka/pkg/apis/messaging/v1alpha1"
	messaginginformers "github.com/knative/eventing/contrib/kafka/pkg/client/informers/externalversions/messaging/v1alpha1"
	listers "github.com/knative/eventing/contrib/kafka/pkg/client/listers/messaging/v1alpha1"
	"github.com/knative/eventing/contrib/kafka/pkg/dispatcher"
	"github.com/knative/eventing/contrib/kafka/pkg/reconciler"
	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/provisioners/fanout"
	"github.com/knative/eventing/pkg/provisioners/multichannelfanout"
	"github.com/knative/pkg/controller"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

const (
	// ReconcilerName is the name of the reconciler.
	ReconcilerName = "KafkaChannels"

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "kafka-ch-dispatcher"

	// Name of the corev1.Events emitted from the reconciliation process.
	channelReconciled         = "ChannelReconciled"
	channelReconcileFailed    = "ChannelReconcileFailed"
	channelUpdateStatusFailed = "ChannelUpdateStatusFailed"
)

// Reconciler reconciles Kafka Channels.
type Reconciler struct {
	*reconciler.Base

	kafkaDispatcher *dispatcher.KafkaDispatcher

	kafkachannelLister   listers.KafkaChannelLister
	kafkachannelInformer cache.SharedIndexInformer
	impl                 *controller.Impl
}

// Check that our Reconciler implements controller.Reconciler.
var _ controller.Reconciler = (*Reconciler)(nil)

// NewController initializes the controller and is called by the generated code.
// Registers event handlers to enqueue events.
func NewController(
	opt reconciler.Options,
	kafkaDispatcher *dispatcher.KafkaDispatcher,
	kafkachannelInformer messaginginformers.KafkaChannelInformer,
) *controller.Impl {

	r := &Reconciler{
		Base:                 reconciler.NewBase(opt, controllerAgentName),
		kafkaDispatcher:      kafkaDispatcher,
		kafkachannelLister:   kafkachannelInformer.Lister(),
		kafkachannelInformer: kafkachannelInformer.Informer(),
	}
	r.impl = controller.NewImpl(r, r.Logger, ReconcilerName)

	r.Logger.Info("Setting up event handlers")

	// Watch for kafka channels.
	kafkachannelInformer.Informer().AddEventHandler(controller.HandleAll(r.impl.Enqueue))

	return r.impl
}

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

	if !original.Status.IsReady() {
		return fmt.Errorf("Channel is not ready. Cannot configure and update subscriber status")
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

	// TODO: Should this check for subscribable status rather than entire status?
	if _, updateStatusErr := r.updateStatus(ctx, channel); updateStatusErr != nil {
		logging.FromContext(ctx).Error("Failed to update KafkaChannel status", zap.Error(updateStatusErr))
		r.Recorder.Eventf(channel, corev1.EventTypeWarning, channelUpdateStatusFailed, "Failed to update KafkaChannel's status: %v", updateStatusErr)
		return updateStatusErr
	}

	// Requeue if the resource is not ready
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, kc *v1alpha1.KafkaChannel) error {
	channels, err := r.kafkachannelLister.List(labels.Everything())
	if err != nil {
		logging.FromContext(ctx).Error("Error listing kafka channels")
		return err
	}

	// TODO: revisit this code. Instead of reading all channels and updating consumers and hostToChannel map for all
	// why not just reconcile the current channel. With this the UpdateKafkaConsumers can now return SubscribableStatus
	// for the subscriptions on the channel that is being reconciled.
	kafkaChannels := make([]*v1alpha1.KafkaChannel, 0)
	for _, channel := range channels {
		if channel.Status.IsReady() {
			kafkaChannels = append(kafkaChannels, channel)
		}
	}
	config := r.newConfigFromKafkaChannels(kafkaChannels)
	if err := r.kafkaDispatcher.UpdateHostToChannelMap(config); err != nil {
		logging.FromContext(ctx).Error("Error updating host to channel map in dispatcher")
		return err
	}

	failedSubscriptions, err := r.kafkaDispatcher.UpdateKafkaConsumers(config)
	if err != nil {
		logging.FromContext(ctx).Error("Error updating kafka consumers in dispatcher")
		return err
	}
	kc.Status.SubscribableTypeStatus.SubscribableStatus = r.createSubscribableStatus(kc.Spec.Subscribable, failedSubscriptions)
	if len(failedSubscriptions) > 0 {
		logging.FromContext(ctx).Error("Some kafka subscriptions failed to subscribe")
		return fmt.Errorf("Some kafka subscriptions failed to subscribe")
	}
	return nil
}

func (r *Reconciler) createSubscribableStatus(subscribable *eventingduck.Subscribable, failedSubscriptions map[eventingduck.SubscriberSpec]error) *eventingduck.SubscribableStatus {
	if subscribable == nil {
		return nil
	}
	subscriberStatus := make([]eventingduck.SubscriberStatus, 0)
	for _, sub := range subscribable.Subscribers {
		status := eventingduck.SubscriberStatus{
			UID:                sub.UID,
			ObservedGeneration: sub.Generation,
			Ready:              corev1.ConditionTrue,
		}
		if err, ok := failedSubscriptions[sub]; ok {
			status.Ready = corev1.ConditionFalse
			status.Message = err.Error()
		}
		subscriberStatus = append(subscriberStatus, status)
	}
	return &eventingduck.SubscribableStatus{
		Subscribers: subscriberStatus,
	}
}

// newConfigFromKafkaChannels creates a new Config from the list of kafka channels.
func (r *Reconciler) newChannelConfigFromKafkaChannel(c *v1alpha1.KafkaChannel) *multichannelfanout.ChannelConfig {
	channelConfig := multichannelfanout.ChannelConfig{
		Namespace: c.Namespace,
		Name:      c.Name,
		HostName:  c.Status.Address.Hostname,
	}
	if c.Spec.Subscribable != nil {
		channelConfig.FanoutConfig = fanout.Config{
			AsyncHandler:  true,
			Subscriptions: c.Spec.Subscribable.Subscribers,
		}
	}
	return &channelConfig
}

// newConfigFromKafkaChannels creates a new Config from the list of kafka channels.
func (r *Reconciler) newConfigFromKafkaChannels(channels []*v1alpha1.KafkaChannel) *multichannelfanout.Config {
	cc := make([]multichannelfanout.ChannelConfig, 0)
	for _, c := range channels {
		channelConfig := r.newChannelConfigFromKafkaChannel(c)
		cc = append(cc, *channelConfig)
	}
	return &multichannelfanout.Config{
		ChannelConfigs: cc,
	}
}
func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.KafkaChannel) (*v1alpha1.KafkaChannel, error) {
	kc, err := r.kafkachannelLister.KafkaChannels(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}

	if reflect.DeepEqual(kc.Status, desired.Status) {
		return kc, nil
	}

	// Don't modify the informers copy.
	existing := kc.DeepCopy()
	existing.Status = desired.Status

	new, err := r.KafkaClientSet.MessagingV1alpha1().KafkaChannels(desired.Namespace).UpdateStatus(existing)
	return new, err
}
