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
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/knative/eventing/contrib/natss/pkg/apis/messaging/v1alpha1"
	clientset "github.com/knative/eventing/contrib/natss/pkg/client/clientset/versioned"
	messaginginformers "github.com/knative/eventing/contrib/natss/pkg/client/informers/externalversions/messaging/v1alpha1"
	listers "github.com/knative/eventing/contrib/natss/pkg/client/listers/messaging/v1alpha1"
	"github.com/knative/eventing/contrib/natss/pkg/dispatcher"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/provisioners/fanout"
	"github.com/knative/eventing/pkg/provisioners/multichannelfanout"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/pkg/controller"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

const (
	// ReconcilerName is the name of the reconciler.
	ReconcilerName = "NatssChannels"

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "natss-ch-dispatcher"

	finalizerName = controllerAgentName
)

// Reconciler reconciles NATSS Channels.
type Reconciler struct {
	*reconciler.Base

	natssDispatcher *dispatcher.SubscriptionsSupervisor

	eventingClientSet    clientset.Interface
	natsschannelLister   listers.NatssChannelLister
	natsschannelInformer cache.SharedIndexInformer
	impl                 *controller.Impl
}

// Check that our Reconciler implements controller.Reconciler.
var _ controller.Reconciler = (*Reconciler)(nil)

// NewController initializes the controller and is called by the generated code.
// Registers event handlers to enqueue events.
func NewController(
	opt reconciler.Options,
	eventingClientSet clientset.Interface,
	natssDispatcher *dispatcher.SubscriptionsSupervisor,
	natsschannelInformer messaginginformers.NatssChannelInformer,
) *controller.Impl {

	r := &Reconciler{
		Base:                 reconciler.NewBase(opt, controllerAgentName),
		eventingClientSet:    eventingClientSet,
		natssDispatcher:      natssDispatcher,
		natsschannelLister:   natsschannelInformer.Lister(),
		natsschannelInformer: natsschannelInformer.Informer(),
	}
	r.impl = controller.NewImpl(r, r.Logger, ReconcilerName)

	r.Logger.Info("Setting up event handlers")

	// Watch for NATSS channels.
	natsschannelInformer.Informer().AddEventHandler(controller.HandleAll(r.impl.Enqueue))

	return r.impl
}

func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key")
		return nil
	}

	// Get the NatssChannel resource with this namespace/name.
	original, err := r.natsschannelLister.NatssChannels(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("NatssChannel key in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy.
	natssChannel := original.DeepCopy()

	// TODO update dispatcher API and use Channelable or NatssChannel.
	c := toChannel(natssChannel)

	// See if the channel has been deleted.
	if natssChannel.DeletionTimestamp != nil {
		if err := r.natssDispatcher.UpdateSubscriptions(c, true); err != nil {
			logging.FromContext(ctx).Error("Error updating subscriptions", zap.Any("channel", c), zap.Error(err))
			return err
		}
		removeFinalizer(natssChannel)
		_, err := r.eventingClientSet.MessagingV1alpha1().NatssChannels(natssChannel.Namespace).Update(natssChannel)
		return err
	}

	// If we are adding the finalizer for the first time, then ensure that finalizer is persisted
	// before manipulating Natss.
	if err := r.ensureFinalizer(natssChannel); err != nil {
		return err
	}

	// Try to subscribe.
	if err := r.natssDispatcher.UpdateSubscriptions(c, false); err != nil {
		logging.FromContext(ctx).Error("Error updating subscriptions", zap.Any("channel", c), zap.Error(err))
		return err
	}

	natssChannels, err := r.natsschannelLister.List(labels.Everything())
	if err != nil {
		logging.FromContext(ctx).Error("Error listing natss channels")
		return err
	}

	channels := make([]eventingv1alpha1.Channel, 0)
	for _, nc := range natssChannels {
		if nc.Status.IsReady() {
			channels = append(channels, *toChannel(nc))
		}
	}

	if err := r.natssDispatcher.UpdateHostToChannelMap(ctx, channels); err != nil {
		logging.FromContext(ctx).Error("Error updating host to channel map", zap.Error(err))
		return err
	}

	return nil
}

// newConfigFromNatssChannels creates a new Config from the list of natss channels.
func (r *Reconciler) newConfigFromNatssChannels(channels []*v1alpha1.NatssChannel) *multichannelfanout.Config {
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

func (r *Reconciler) ensureFinalizer(channel *v1alpha1.NatssChannel) error {
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

	_, err = r.eventingClientSet.MessagingV1alpha1().NatssChannels(channel.Namespace).Patch(channel.Name, types.MergePatchType, patch)
	return err
}

func removeFinalizer(channel *v1alpha1.NatssChannel) {
	finalizers := sets.NewString(channel.Finalizers...)
	finalizers.Delete(finalizerName)
	channel.Finalizers = finalizers.List()
}

func toChannel(natssChannel *v1alpha1.NatssChannel) *eventingv1alpha1.Channel {
	return &eventingv1alpha1.Channel{
		ObjectMeta: v1.ObjectMeta{
			Name:      natssChannel.Name,
			Namespace: natssChannel.Namespace,
		},
		Spec: eventingv1alpha1.ChannelSpec{
			Subscribable: natssChannel.Spec.Subscribable,
		},
		Status: eventingv1alpha1.ChannelStatus{
			Address: natssChannel.Status.Address,
		},
	}
}
