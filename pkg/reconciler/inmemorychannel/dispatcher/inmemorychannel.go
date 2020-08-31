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

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"

	v1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/channel/fanout"
	"knative.dev/eventing/pkg/channel/multichannelfanout"
	listers "knative.dev/eventing/pkg/client/listers/messaging/v1"
	"knative.dev/eventing/pkg/inmemorychannel"
)

// Reconciler reconciles InMemory Channels.
type Reconciler struct {
	eventDispatcherConfigStore *channel.EventDispatcherConfigStore
	dispatcher                 inmemorychannel.MessageDispatcher
	inmemorychannelLister      listers.InMemoryChannelLister
	inmemorychannelInformer    cache.SharedIndexInformer
}

func (r *Reconciler) ReconcileKind(ctx context.Context, imc *v1.InMemoryChannel) reconciler.Event {
	// This is a special Reconciler that does the following:
	// 1. Lists the inmemory channels.
	// 2. Creates a multi-channel-fanout-config.
	// 3. Calls the inmemory channel dispatcher's updateConfig func with the new multi-channel-fanout-config.
	channels, err := r.inmemorychannelLister.List(labels.Everything())
	if err != nil {
		logging.FromContext(ctx).Error("Error listing InMemory channels")
		return err
	}

	inmemoryChannels := make([]*v1.InMemoryChannel, 0)
	for _, imc := range channels {
		if imc.Status.IsReady() {
			inmemoryChannels = append(inmemoryChannels, imc)
		}
	}

	config, err := r.newConfigFromInMemoryChannels(inmemoryChannels)
	if err != nil {
		logging.FromContext(ctx).Error("Error creating config from in memory channels")
		return err
	}
	err = r.dispatcher.UpdateConfig(ctx, r.eventDispatcherConfigStore.GetConfig(), config)
	if err != nil {
		logging.FromContext(ctx).Error("Error updating InMemory dispatcher config")
		return err
	}

	return nil
}

// newConfigFromInMemoryChannels creates a new Config from the list of inmemory channels.
func (r *Reconciler) newConfigFromInMemoryChannels(channels []*v1.InMemoryChannel) (*multichannelfanout.Config, error) {
	cc := make([]multichannelfanout.ChannelConfig, 0)
	for _, c := range channels {

		subs := make([]fanout.Subscription, len(c.Spec.Subscribers))
		for _, sub := range c.Spec.Subscribers {
			conf, err := fanout.SubscriberSpecToFanoutConfig(sub)
			if err != nil {
				return nil, err
			}

			subs = append(subs, *conf)
		}

		channelConfig := multichannelfanout.ChannelConfig{
			Namespace: c.Namespace,
			Name:      c.Name,
			HostName:  c.Status.Address.URL.Host,
			FanoutConfig: fanout.Config{
				AsyncHandler:  true,
				Subscriptions: subs,
			},
		}
		cc = append(cc, channelConfig)
	}
	return &multichannelfanout.Config{
		ChannelConfigs: cc,
	}, nil
}
