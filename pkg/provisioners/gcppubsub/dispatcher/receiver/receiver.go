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
	"fmt"

	"github.com/knative/pkg/logging"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/knative/eventing/pkg/provisioners"

	"cloud.google.com/go/pubsub"
	"github.com/knative/eventing/pkg/provisioners/gcppubsub/dispatcher/receiver/cache"
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

	cache *cache.TTL
}

// New creates a new Receiver and its associated MessageReceiver. The caller is responsible for
// Start()ing the returned MessageReceiver.
func New(logger *zap.Logger, client client.Client, pubSubClientCreator util.PubSubClientCreator, defaultGcpProject string, defaultSecret *v1.ObjectReference, defaultSecretKey string) (*Receiver, []manager.Runnable) {
	r := &Receiver{
		logger: logger,
		client: client,

		pubSubClientCreator: pubSubClientCreator,

		defaultGcpProject: defaultGcpProject,
		defaultSecret:     defaultSecret,
		defaultSecretKey:  defaultSecretKey,

		cache: cache.NewTTL(),
	}
	return r, []manager.Runnable{r.newMessageReceiver(), r.cache}
}

func (r *Receiver) newMessageReceiver() *provisioners.MessageReceiver {
	return provisioners.NewMessageReceiver(r.sendEventToTopic, r.logger.Sugar())
}

// sendEventToTopic sends a message to the Cloud Pub/Sub Topic backing the Channel.
func (r *Receiver) sendEventToTopic(channel provisioners.ChannelReference, message *provisioners.Message) error {
	r.logger.Info("received message")
	ctx := context.Background()

	c, err := r.getChannel(ctx, channel)
	if err != nil {
		logging.FromContext(ctx).Info("Unable to get the Channel", zap.Error(err), zap.Any("channelRef", channel))
		return err
	}
	pbs, err := util.ReadRawStatus(ctx, c)
	if err != nil {
		return err
	}

	cached := r.cache.Get(pbs.Topic)
	if cached == nil {
		cached, err = r.createPubSubReceiver(ctx, pbs)
		if err != nil {
			return err
		}
	}

	psr, ok := cached.(*pubSubReceiver)
	if !ok {
		return fmt.Errorf("unexpected type in the cache[%s] - %T", pbs.Topic, cached)
	}
	result := psr.topic.Publish(ctx, &pubsub.Message{
		Data:       message.Payload,
		Attributes: message.Headers,
	})

	id, err := result.Get(ctx)
	if err != nil {
		return err
	}

	r.logger.Info("Published a message", zap.String("topicName", pbs.Topic), zap.String("pubSubMessageId", id))
	return nil
}

func (r *Receiver) createPubSubReceiver(ctx context.Context, pbs *util.GcpPubSubChannelStatus) (cache.Stoppable, error) {

	creds, err := util.GetCredentials(ctx, r.client, r.defaultSecret, r.defaultSecretKey)
	if err != nil {
		r.logger.Info("Failed to extract creds", zap.Error(err))
		return nil, err
	}

	psc, err := r.pubSubClientCreator(ctx, creds, r.defaultGcpProject)
	if err != nil {
		r.logger.Info("Failed to create PubSub client", zap.Error(err))
		return nil, err
	}
	topic := psc.Topic(pbs.Topic)
	psr := r.cache.Insert(pbs.Topic, &pubSubReceiver{
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
