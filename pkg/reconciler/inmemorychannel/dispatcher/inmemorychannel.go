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

	"github.com/knative/eventing/pkg/inmemorychannel"

	"github.com/knative/eventing/pkg/apis/messaging/v1alpha1"
	messaginginformers "github.com/knative/eventing/pkg/client/informers/externalversions/messaging/v1alpha1"
	listers "github.com/knative/eventing/pkg/client/listers/messaging/v1alpha1"
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
	ReconcilerName = "InMemoryChannels"

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "in-memory-channel-dispatcher"
)

// Reconciler reconciles InMemory Channels.
type Reconciler struct {
	*reconciler.Base

	dispatcher              inmemorychannel.Dispatcher
	inmemorychannelLister   listers.InMemoryChannelLister
	inmemorychannelInformer cache.SharedIndexInformer
	impl                    *controller.Impl
}

// Check that our Reconciler implements controller.Reconciler.
var _ controller.Reconciler = (*Reconciler)(nil)

// NewController initializes the controller and is called by the generated code.
// Registers event handlers to enqueue events.
func NewController(
	opt reconciler.Options,
	dispatcher inmemorychannel.Dispatcher,
	inmemorychannelinformer messaginginformers.InMemoryChannelInformer,
) *controller.Impl {

	r := &Reconciler{
		Base:                    reconciler.NewBase(opt, controllerAgentName),
		dispatcher:              dispatcher,
		inmemorychannelLister:   inmemorychannelinformer.Lister(),
		inmemorychannelInformer: inmemorychannelinformer.Informer(),
	}
	r.impl = controller.NewImpl(r, r.Logger, ReconcilerName)

	r.Logger.Info("Setting up event handlers")

	// Watch for inmemory channels.
	r.inmemorychannelInformer.AddEventHandler(controller.HandleAll(r.impl.Enqueue))

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
	// 1. Lists the inmemory channels.
	// 2. Creates a multi-channel-fanout-config.
	// 3. Calls the inmemory channel dispatcher's updateConfig func with the new multi-channel-fanout-config.

	channels, err := r.inmemorychannelLister.List(labels.Everything())
	if err != nil {
		logging.FromContext(ctx).Error("Error listing InMemory channels")
		return err
	}

	inmemoryChannels := make([]*v1alpha1.InMemoryChannel, 0)
	for _, channel := range channels {
		if channel.Status.IsReady() {
			inmemoryChannels = append(inmemoryChannels, channel)
		}
	}

	config := r.newConfigFromInMemoryChannels(inmemoryChannels)
	err = r.dispatcher.UpdateConfig(config)
	if err != nil {
		logging.FromContext(ctx).Error("Error updating InMemory dispatcher config")
		return err
	}

	return nil
}

// newConfigFromInMemoryChannels creates a new Config from the list of inmemory channels.
func (r *Reconciler) newConfigFromInMemoryChannels(channels []*v1alpha1.InMemoryChannel) *multichannelfanout.Config {
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
