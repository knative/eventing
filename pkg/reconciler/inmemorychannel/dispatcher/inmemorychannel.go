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

	"knative.dev/pkg/reconciler"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	duckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/channel/fanout"
	"knative.dev/eventing/pkg/channel/multichannelfanout"
	listers "knative.dev/eventing/pkg/client/listers/messaging/v1alpha1"
	"knative.dev/eventing/pkg/inmemorychannel"
	"knative.dev/eventing/pkg/logging"
)

// Reconciler reconciles InMemory Channels.
type Reconciler struct {
	configStore             *channel.EventDispatcherConfigStore
	dispatcher              inmemorychannel.MessageDispatcher
	inmemorychannelLister   listers.InMemoryChannelLister
	inmemorychannelInformer cache.SharedIndexInformer
}

func (r *Reconciler) ReconcileKind(ctx context.Context, imc *v1alpha1.InMemoryChannel) reconciler.Event {
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
	for _, imc := range channels {
		if imc.Status.IsReady() {
			inmemoryChannels = append(inmemoryChannels, imc)
		}
	}

	config := r.newConfigFromInMemoryChannels(inmemoryChannels)
	err = r.dispatcher.UpdateConfig(ctx, config)
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
			// Translate subs from v1alpha1 to v1beta1
			subs := make([]duckv1beta1.SubscriberSpec, len(c.Spec.Subscribable.Subscribers))
			for i, s := range c.Spec.Subscribable.Subscribers {
				s.ConvertTo(context.TODO(), &subs[i])
			}
			channelConfig.FanoutConfig = fanout.Config{
				AsyncHandler:     true,
				Subscriptions:    subs,
				DispatcherConfig: r.configStore.GetConfig(),
			}
		}
		cc = append(cc, channelConfig)
	}
	return &multichannelfanout.Config{
		ChannelConfigs: cc,
	}
}
