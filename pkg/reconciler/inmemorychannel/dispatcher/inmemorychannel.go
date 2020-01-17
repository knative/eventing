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

package dispatcher

import (
	"context"
	"fmt"

	"knative.dev/eventing/pkg/inmemorychannel"

	"go.uber.org/zap"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/eventing/pkg/channel/fanout"
	"knative.dev/eventing/pkg/channel/multichannelfanout"
	listers "knative.dev/eventing/pkg/client/listers/messaging/v1alpha1"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/pkg/controller"
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

func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key")
		return nil
	}

	// Get the IMC resource with this namespace/name.
	channel, err := r.inmemorychannelLister.InMemoryChannels(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("InMemoryChannel key in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	if !channel.Status.IsReady() {
		return fmt.Errorf("Channel is not ready. Cannot configure the dispatcher")
	}

	// Just update the dispatcher config
	reconcileErr := r.reconcile(ctx, channel)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling InMemoryChannel", zap.Error(reconcileErr))
	} else {
		logging.FromContext(ctx).Debug("InMemoryChannel reconciled")
	}
	return nil
}

func (r *Reconciler) reconcile(ctx context.Context, imc *v1alpha1.InMemoryChannel) error {
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
