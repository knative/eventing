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

	"github.com/knative/eventing/pkg/provisioners"
	v1 "k8s.io/api/core/v1"

	"cloud.google.com/go/pubsub"
	"github.com/knative/eventing/pkg/provisioners/gcppubsub/util"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Receiver parses Cloud Events and sends them to GCP PubSub.
type Receiver struct {
	logger *zap.Logger
	client client.Client

	pubSubClientCreator util.PubSubClientCreator

	// Note that for all the default* parameters below, these must be kept in lock-step with the
	// GCP PubSub Dispatcher's reconciler.
	// Eventually, individual Channels should be allowed to specify different projects and secrets,
	// but for now all Channels use the same project and secret.

	// defaultGcpProject is the GCP project ID where PubSub Topics and Subscriptions are created.
	defaultGcpProject string
	// defaultSecret and defaultSecretKey are the K8s Secret and key in that secret that contain a
	// JSON format GCP service account token, see
	// https://cloud.google.com/iam/docs/creating-managing-service-account-keys#iam-service-account-keys-create-gcloud
	defaultSecret    *v1.ObjectReference
	defaultSecretKey string
}

// New creates a new Receiver and its associated MessageReceiver. The caller is responsible for
// Start()ing the returned MessageReceiver.
func New(logger *zap.Logger, client client.Client, pubSubClientCreator util.PubSubClientCreator, defaultGcpProject string, defaultSecret *v1.ObjectReference, defaultSecretKey string) (*Receiver, *provisioners.MessageReceiver) {
	r := &Receiver{
		logger: logger,
		client: client,

		pubSubClientCreator: pubSubClientCreator,

		defaultGcpProject: defaultGcpProject,
		defaultSecret:     defaultSecret,
		defaultSecretKey:  defaultSecretKey,
	}
	return r, r.newMessageReceiver()
}

func (r *Receiver) newMessageReceiver() *provisioners.MessageReceiver {
	return provisioners.NewMessageReceiver(r.sendEventToTopic, r.logger.Sugar())
}

// sendEventToTopic sends a message to the Cloud Pub/Sub Topic backing the Channel.
func (r *Receiver) sendEventToTopic(channel provisioners.ChannelReference, message *provisioners.Message) error {
	r.logger.Debug("received message")
	ctx := context.Background()

	creds, err := util.GetCredentials(ctx, r.client, r.defaultSecret, r.defaultSecretKey)
	if err != nil {
		r.logger.Error("Failed to extract creds", zap.Error(err))
		return err
	}

	psc, err := r.pubSubClientCreator(ctx, creds, r.defaultGcpProject)
	if err != nil {
		r.logger.Error("Failed to create PubSub client", zap.Error(err))
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

	r.logger.Debug("Published a message", zap.String("topicName", topicName), zap.String("pubSubMessageId", id))
	return nil
}
