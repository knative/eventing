/*
 * Copyright 2018 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package receiver

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/pubsub"
	"github.com/knative/eventing/contrib/gcppubsub/pkg/dispatcher/receiver/cache"
	"github.com/knative/eventing/contrib/gcppubsub/pkg/util"
	"github.com/knative/eventing/contrib/gcppubsub/pkg/util/logging"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Receiver parses Cloud Events and sends them to GCP PubSub.
type Receiver struct {
	logger *zap.Logger
	client client.Client

	pubSubClientCreator util.PubSubClientCreator
	cache               *cache.TTL
}

// New creates a new Receiver and its associated MessageReceiver. The caller is responsible for
// Start()ing the returned MessageReceiver.
func New(logger *zap.Logger, client client.Client, pubSubClientCreator util.PubSubClientCreator) (*Receiver, []manager.Runnable) {
	r := &Receiver{
		logger: logger,
		client: client,

		pubSubClientCreator: pubSubClientCreator,
		cache:               cache.NewTTL(),
	}
	return r, []manager.Runnable{r.newMessageReceiver(), r.cache}
}

func (r *Receiver) newMessageReceiver() *provisioners.MessageReceiver {
	return provisioners.NewMessageReceiver(r.sendEventToTopic, r.logger.Sugar())
}

// sendEventToTopic sends a message to the Cloud Pub/Sub Topic backing the Channel.
func (r *Receiver) sendEventToTopic(channel provisioners.ChannelReference, message *provisioners.Message) error {
	r.logger.Debug("received message")
	ctx := context.Background()

	c, err := r.getChannel(ctx, channel)
	if err != nil {
		logging.FromContext(ctx).Info("Unable to get the Channel", zap.Error(err), zap.Any("channelRef", channel))
		return err
	}
	pcs, err := util.GetInternalStatus(c)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to read status.internal", zap.Error(err), zap.Any("channel", c))
		return err
	} else if pcs.IsEmpty() {
		logging.FromContext(ctx).Info("status.internal is blank")
		return errors.New("status.internal is blank")
	}

	cached := r.cache.Get(pcs.Topic)
	if cached == nil {
		cached, err = r.createPubSubReceiver(ctx, pcs)
		if err != nil {
			logging.FromContext(ctx).Info("Unable to create pubSubReceiver", zap.Error(err))
			return err
		}
	}

	psr, ok := cached.(*pubSubReceiver)
	if !ok {
		r.cache.DeleteIfPresent(pcs.Topic, psr)
		return fmt.Errorf("unexpected type in the cache[%s] - %T", pcs.Topic, cached)
	}

	result := psr.topic.Publish(ctx, &pubsub.Message{
		Data:       message.Payload,
		Attributes: message.Headers,
	})

	id, err := result.Get(ctx)
	if err != nil {
		return err
	}

	r.logger.Debug("Published a message", zap.String("topicName", pcs.Topic), zap.String("pubSubMessageId", id))
	return nil
}

func (r *Receiver) createPubSubReceiver(ctx context.Context, pcs *util.GcpPubSubChannelStatus) (cache.Stoppable, error) {
	creds, err := util.GetCredentials(ctx, r.client, pcs.Secret, pcs.SecretKey)
	if err != nil {
		r.logger.Info("Failed to extract creds", zap.Error(err))
		return nil, err
	}

	psc, err := r.pubSubClientCreator(ctx, creds, pcs.GCPProject)
	if err != nil {
		r.logger.Info("Failed to create PubSub client", zap.Error(err))
		return nil, err
	}
	// Note that creating topic does not actually start any background work. So this struct can be
	// discarded without any problems. Once topic.Publish is called, then background work starts and
	// topic.Stop must be called eventually.
	topic := psc.Topic(pcs.Topic)
	psr := r.cache.Insert(pcs.Topic, &pubSubReceiver{
		topic: topic,
	})
	return psr, nil
}

type pubSubReceiver struct {
	topic util.PubSubTopic
}

func (psr *pubSubReceiver) Stop() {
	psr.topic.Stop()
}

func (r *Receiver) getChannel(ctx context.Context, ref provisioners.ChannelReference) (*eventingv1alpha1.Channel, error) {
	c := &eventingv1alpha1.Channel{}
	err := r.client.Get(ctx,
		types.NamespacedName{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		},
		c)

	return c, err
}
