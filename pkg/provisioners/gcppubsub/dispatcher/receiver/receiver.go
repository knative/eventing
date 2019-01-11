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

	"cloud.google.com/go/pubsub"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners"
	"github.com/knative/eventing/pkg/provisioners/gcppubsub/util"
	"github.com/knative/eventing/pkg/provisioners/gcppubsub/util/logging"
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
}

// New creates a new Receiver and its associated MessageReceiver. The caller is responsible for
// Start()ing the returned MessageReceiver.
func New(logger *zap.Logger, client client.Client, pubSubClientCreator util.PubSubClientCreator) (*Receiver, []manager.Runnable) {
	r := &Receiver{
		logger: logger,
		client: client,

		pubSubClientCreator: pubSubClientCreator,
	}
	return r, []manager.Runnable{r.newMessageReceiver()}
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
	pbs, err := util.ReadRawStatus(ctx, c)
	if err != nil {
		return err
	} else if pbs.IsEmpty() {
		logging.FromContext(ctx).Info("Raw status is blank")
		return errors.New("raw status is blank")
	}

	psr, err := r.createPubSubReceiver(ctx, pbs)
	if err != nil {
		logging.FromContext(ctx).Info("Unable to create pubSubReceiver", zap.Error(err))
		return err
	}

	result := psr.Publish(ctx, &pubsub.Message{
		Data:       message.Payload,
		Attributes: message.Headers,
	})

	id, err := result.Get(ctx)
	if err != nil {
		return err
	}
	// TODO allow topics to be reused between publish events, call .Stop after an idle period.
	psr.Stop()

	r.logger.Debug("Published a message", zap.String("topicName", pbs.Topic), zap.String("pubSubMessageId", id))
	return nil
}

func (r *Receiver) createPubSubReceiver(ctx context.Context, pbs *util.GcpPubSubChannelStatus) (util.PubSubTopic, error) {
	creds, err := util.GetCredentials(ctx, r.client, pbs.Secret, pbs.SecretKey)
	if err != nil {
		r.logger.Info("Failed to extract creds", zap.Error(err))
		return nil, err
	}

	psc, err := r.pubSubClientCreator(ctx, creds, pbs.GCPProject)
	if err != nil {
		r.logger.Info("Failed to create PubSub client", zap.Error(err))
		return nil, err
	}
	return psc.Topic(pbs.Topic), nil
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
