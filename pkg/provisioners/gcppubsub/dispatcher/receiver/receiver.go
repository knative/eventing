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

	"cloud.google.com/go/pubsub"
	"github.com/knative/eventing/pkg/buses"
	"github.com/knative/eventing/pkg/provisioners/gcppubsub/util"
	"go.uber.org/zap"
	"k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Receiver struct {
	logger *zap.Logger
	client client.Client

	pubSubClientCreator util.PubSubClientCreator

	defaultGcpProject string
	defaultSecret     v1.ObjectReference
	defaultSecretKey  string
}

func New(logger *zap.Logger, client client.Client, pubSubClientCreator util.PubSubClientCreator, defaultGcpProject string, defaultSecret v1.ObjectReference, defaultSecretKey string) *Receiver {
	return &Receiver{
		logger: logger,
		client: client,

		pubSubClientCreator: pubSubClientCreator,

		defaultGcpProject: defaultGcpProject,
		defaultSecret:     defaultSecret,
		defaultSecretKey:  defaultSecretKey,
	}
}

func (r *Receiver) NewMessageReceiver() *buses.MessageReceiver {
	return buses.NewMessageReceiver(r.sendEventToTopic, r.logger.Sugar())
}

// sendEventToTopic sends a message to the Cloud Pub/Sub Topic backing the
// Channel.
func (r *Receiver) sendEventToTopic(channel buses.ChannelReference, message *buses.Message) error {
	r.logger.Info("received message")
	ctx := context.Background()

	creds, err := util.GetCredentials(ctx, r.client, r.defaultSecret, r.defaultSecretKey)
	if err != nil {
		r.logger.Info("Failed to extract creds", zap.Error(err))
		return err
	}

	psc, err := r.pubSubClientCreator(ctx, creds, r.defaultGcpProject)
	if err != nil {
		r.logger.Info("Failed to create PubSub client", zap.Error(err))
		return err
	}
	topicName := util.GenerateTopicName(channel.Namespace, channel.Name)
	topic := psc.Topic(topicName)
	result := topic.Publish(ctx, &pubsub.Message{
		Data:       message.Payload,
		Attributes: message.Headers,
	})

	id, err := result.Get(ctx)
	if err != nil {
		return err
	}

	// TODO allow topics to be reused between publish events, call .Stop after an idle period
	topic.Stop()

	r.logger.Info("Published a message", zap.String("topicName", topicName), zap.String("pubSubMessageId", id))
	return nil
}
