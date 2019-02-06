/*
Copyright 2018 The Knative Authors

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
	"sync"
	"time"

	"k8s.io/client-go/util/workqueue"

	"sigs.k8s.io/controller-runtime/pkg/event"

	pubsubutil "github.com/knative/eventing/contrib/gcppubsub/pkg/util"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// controllerAgentName is the string used by this controller to identify itself when creating
	// events.
	controllerAgentName = "gcp-pubsub-channel-dispatcher"

	// Exponential backoff constants used to limit the pace at which
	// we nack messages in order to throttle the channel retries.
	// The backoff is computed as follows: min(expBackoffMaxDelay, expBackoffBaseDelay* 2^#failures).
	// expBackoffMaxDelay should be less than subscription.ReceiveSettings.MaxExtension,
	// which is the maximum period for which the Subscription extends the ack deadline
	// for each message. It is currently set to 10 minutes.
	expBackoffBaseDelay = 1 * time.Second
	expBackoffMaxDelay  = 5 * time.Minute
)

// New returns a Controller that represents the dispatcher portion (messages from GCP PubSub are
// sent into the cluster) of the GCP PubSub dispatcher. We use a reconcile loop to watch all
// Channels and notice changes to them. It uses an exponential backoff to throttle the retries.
func New(mgr manager.Manager, logger *zap.Logger, stopCh <-chan struct{}) (controller.Controller, error) {
	// reconcileChan is used when the dispatcher itself needs to force reconciliation of a Channel.
	reconcileChan := make(chan event.GenericEvent)

	// Setup a new controller to pull messages from GCP PubSub for Channels that belong to this
	// Cluster Provisioner (gcp-pubsub).
	r := &reconciler{
		recorder: mgr.GetRecorder(controllerAgentName),
		logger:   logger,

		dispatcher:    provisioners.NewMessageDispatcher(logger.Sugar()),
		reconcileChan: reconcileChan,

		pubSubClientCreator: pubsubutil.GcpPubSubClientCreator,

		subscriptionsLock: sync.Mutex{},
		subscriptions:     map[channelName]map[subscriptionName]context.CancelFunc{},

		rateLimiter: workqueue.NewItemExponentialFailureRateLimiter(expBackoffBaseDelay, expBackoffMaxDelay),
	}

	c, err := controller.New(controllerAgentName, mgr, controller.Options{
		Reconciler: r,
	})
	if err != nil {
		logger.Error("Unable to create controller.", zap.Error(err))
		return nil, err
	}

	// Watch Channels.
	err = c.Watch(&source.Kind{
		Type: &eventingv1alpha1.Channel{},
	}, &handler.EnqueueRequestForObject{})
	if err != nil {
		logger.Error("Unable to watch Channels.", zap.Error(err), zap.Any("type", &eventingv1alpha1.Channel{}))
		return nil, err
	}

	// The PubSub library may fail when receiving messages. If it does so, then we need to reconcile
	// that Channel again.
	// TODO Once https://github.com/kubernetes-sigs/controller-runtime/issues/103 is fixed, switch
	// to:
	// err = c.Watch(&source.Channel{
	// 	Source: reconcileChan,
	// }, &handler.EnqueueRequestForOwner{})
	src := &source.Channel{
		Source: reconcileChan,
	}
	err = src.InjectStopChannel(stopCh)
	if err != nil {
		logger.Error("Unable to inject the stop channel", zap.Error(err))
		return nil, err
	}
	err = c.Watch(src, &handler.EnqueueRequestForObject{})
	if err != nil {
		logger.Error("Unable to watch the reconcile Channel", zap.Error(err))
		return nil, err
	}

	return c, nil
}
