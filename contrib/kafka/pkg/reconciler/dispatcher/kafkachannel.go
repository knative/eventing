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
	"github.com/knative/eventing/contrib/kafka/pkg/apis/messaging/v1alpha1"
	clientset "github.com/knative/eventing/contrib/kafka/pkg/client/clientset/versioned"
	messaginginformers "github.com/knative/eventing/contrib/kafka/pkg/client/informers/externalversions/messaging/v1alpha1"
	listers "github.com/knative/eventing/contrib/kafka/pkg/client/listers/messaging/v1alpha1"
	"github.com/knative/eventing/contrib/kafka/pkg/dispatcher"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/provisioners/fanout"
	"github.com/knative/eventing/pkg/provisioners/multichannelfanout"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/pkg/controller"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

const (
	// ReconcilerName is the name of the reconciler.
	ReconcilerName = "KafkaChannels"

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "kafka-ch-dispatcher"
)

// Reconciler reconciles Kafka Channels.
type Reconciler struct {
	*reconciler.Base

	kafkaDispatcher *dispatcher.KafkaDispatcher

	eventingClientSet    clientset.Interface
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
	eventingClientSet clientset.Interface,
	kafkaDispatcher *dispatcher.KafkaDispatcher,
	kafkachannelInformer messaginginformers.KafkaChannelInformer,
) *controller.Impl {

	r := &Reconciler{
		Base:                 reconciler.NewBase(opt, controllerAgentName),
		kafkaDispatcher:      kafkaDispatcher,
		eventingClientSet:    eventingClientSet,
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
	_, _, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key")
		return nil
	}

	// This is a special Reconciler that does the following:
	// 1. Lists the kafka channels.
	// 2. Creates a multi-channel-fanout-config.
	// 3. Calls the kafka dispatcher's updateConfig func with the new multi-channel-fanout-config.

	channels, err := r.kafkachannelLister.List(labels.Everything())
	if err != nil {
		logging.FromContext(ctx).Error("Error listing kafka channels")
		return err
	}

	kafkaChannels := make([]*v1alpha1.KafkaChannel, 0)
	for _, channel := range channels {
		if channel.Status.IsReady() {
			kafkaChannels = append(kafkaChannels, channel)
		}
	}

	config := r.newConfigFromKafkaChannels(kafkaChannels)
	err = r.kafkaDispatcher.UpdateConfig(config)
	if err != nil {
		logging.FromContext(ctx).Error("Error updating kafka dispatcher config")
		return err
	}

	return nil
}

// newConfigFromKafkaChannels creates a new Config from the list of kafka channels.
func (r *Reconciler) newConfigFromKafkaChannels(channels []*v1alpha1.KafkaChannel) *multichannelfanout.Config {
	cc := make([]multichannelfanout.ChannelConfig, 0)
	for _, c := range channels {
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
		cc = append(cc, channelConfig)
	}
	return &multichannelfanout.Config{
		ChannelConfigs: cc,
	}
}
