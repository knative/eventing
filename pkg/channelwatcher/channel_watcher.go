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
package channelwatcher

import (
	"context"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
	"github.com/knative/eventing/pkg/sidecar/swappable"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type reconciler struct {
	client  client.Client
	logger  *zap.Logger
	handler WatchHandlerFunc
}

func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	ctx := logging.WithLogger(context.TODO(), r.logger.With(zap.Any("request", req)))
	logging.FromContext(ctx).Info("New update for channel.")
	if err := r.handler(ctx, r.client, req.NamespacedName); err != nil {
		logging.FromContext(ctx).Error("WatchHandlerFunc returned error", zap.Error(err))
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

// New creates a new instance of Channel Watcher that watches channels and calls the watchHandler on add, update, delete and generic event
func New(mgr manager.Manager, logger *zap.Logger, watchHandler WatchHandlerFunc) error {
	c, err := controller.New("ChannelWatcher", mgr, controller.Options{
		Reconciler: &reconciler{
			client:  mgr.GetClient(),
			logger:  logger,
			handler: watchHandler,
		},
	})
	if err != nil {
		logger.Error("Unable to create controller for channelwatcher.", zap.Error(err))
		return err
	}

	// Watch Channels.
	err = c.Watch(&source.Kind{
		Type: &v1alpha1.Channel{},
	}, &handler.EnqueueRequestForObject{})
	if err != nil {
		logger.Error("Unable to watch Channels.", zap.Error(err), zap.Any("type", &v1alpha1.Channel{}))
		return err
	}
	return nil
}

// WatchHandlerFunc is called whenever an add, update, delete or generic event is triggered on a channel
type WatchHandlerFunc func(context.Context, client.Client, types.NamespacedName) error

// ShouldWatchFunc is called while returning list of channels.
// Channels are included in the list if the return value is true.
type ShouldWatchFunc func(ch *v1alpha1.Channel) bool

// UpdateConfigWatchHandler is a special handler that
// 1. Lists the channels for which shouldWatch returns true.
// 2. Creates a multi-channel-fanout-config.
// 3. Calls the updateConfig func with the new multi-channel-fanout-config.
// This is used by dispatchers or receivers to update their configs by watching channels.
func UpdateConfigWatchHandler(updateConfig swappable.UpdateConfig, shouldWatch ShouldWatchFunc) WatchHandlerFunc {
	return func(ctx context.Context, c client.Client, _ types.NamespacedName) error {
		channels, err := ListAllChannels(ctx, c, shouldWatch)
		if err != nil {
			logging.FromContext(ctx).Info("Unable to list channels", zap.Error(err))
			return err
		}
		config := multichannelfanout.NewConfigFromChannels(channels)
		return updateConfig(config)
	}
}

// ListAllChannels queries client and gets list of all channels for which shouldWatch returns true.
func ListAllChannels(ctx context.Context, c client.Client, shouldWatch ShouldWatchFunc) ([]v1alpha1.Channel, error) {
	channels := make([]v1alpha1.Channel, 0)
	cl := &v1alpha1.ChannelList{}
	if err := c.List(ctx, &client.ListOptions{}, cl); err != nil {
		return nil, err
	}
	for _, c := range cl.Items {
		if c.Status.IsReady() && shouldWatch(&c) {
			channels = append(channels, c)
		}
	}
	return channels, nil
}
