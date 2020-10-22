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

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/equality"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"

	v1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/channel/fanout"
	"knative.dev/eventing/pkg/channel/multichannelfanout"
)

// Reconciler reconciles InMemory Channels.
type Reconciler struct {
	eventDispatcherConfigStore *channel.EventDispatcherConfigStore
	multiChannelMessageHandler multichannelfanout.MultiChannelMessageHandler
	reporter                   channel.StatsReporter
}

func (r *Reconciler) ReconcileKind(ctx context.Context, imc *v1.InMemoryChannel) reconciler.Event {
	logging.FromContext(ctx).Infow("Reconciling", zap.Any("InMemoryChannel", imc))

	if !imc.Status.IsReady() {
		logging.FromContext(ctx).Debug("IMC is not ready, skipping")
		return nil
	}

	config, err := r.newConfigForInMemoryChannel(imc)
	if err != nil {
		logging.FromContext(ctx).Error("Error creating config for in memory channels", zap.Error(err))
		return err
	}

	// First grab the MultiChannelFanoutMessage handler
	handler := r.multiChannelMessageHandler.GetChannelHandler(config.HostName)
	if handler == nil {
		// No handler yet, create one.
		fanoutHandler, err := fanout.NewFanoutMessageHandler(logging.FromContext(ctx).Desugar(), channel.NewMessageDispatcher(logging.FromContext(ctx).Desugar()), config.FanoutConfig, r.reporter)
		if err != nil {
			logging.FromContext(ctx).Error("Failed to create a new fanout.MessageHandler", err)
			return err
		}
		r.multiChannelMessageHandler.SetChannelHandler(config.HostName, fanoutHandler)
	} else {
		// Just update the config if necessary.
		haveSubs := handler.GetSubscriptions(ctx)
		for _, s := range haveSubs {
			logging.FromContext(ctx).Info("Have subscription", zap.String("have", s.String()))
		}
		for _, s := range config.FanoutConfig.Subscriptions {
			logging.FromContext(ctx).Info("Want subscription", zap.String("want", s.String()))
		}

		if !equality.Semantic.DeepDerivative(config.FanoutConfig.Subscriptions, haveSubs) {
			logging.FromContext(ctx).Info("Updating fanout config (-old +new)", zap.Any("want", config.FanoutConfig.Subscriptions), zap.Any("have", haveSubs))
			handler.SetSubscriptions(ctx, config.FanoutConfig.Subscriptions)
		}
	}
	return nil
}

func (r *Reconciler) FinalizeKind(ctx context.Context, imc *v1.InMemoryChannel) reconciler.Event {
	config, err := r.newConfigForInMemoryChannel(imc)
	if err != nil {
		logging.FromContext(ctx).Error("Error creating config for in memory channels", zap.Error(err))
		return err
	}
	logging.FromContext(ctx).Info("Removing dispatcher")
	r.multiChannelMessageHandler.DeleteChannelHandler(config.HostName)
	return nil
}

// newConfigForInMemoryChannel creates a new Config for a single inmemory channel.
func (r *Reconciler) newConfigForInMemoryChannel(imc *v1.InMemoryChannel) (*multichannelfanout.ChannelConfig, error) {
	subs := make([]fanout.Subscription, len(imc.Spec.Subscribers))

	for i, sub := range imc.Spec.Subscribers {
		conf, err := fanout.SubscriberSpecToFanoutConfig(sub)
		if err != nil {
			return nil, err
		}
		if conf.DeadLetter == nil && conf.Reply == nil && conf.Subscriber == nil {
			continue
		}
		subs[i] = *conf
	}

	return &multichannelfanout.ChannelConfig{
		Namespace: imc.Namespace,
		Name:      imc.Name,
		HostName:  imc.Status.Address.URL.Host,
		FanoutConfig: fanout.Config{
			AsyncHandler:  true,
			Subscriptions: subs,
		},
	}, nil
}
