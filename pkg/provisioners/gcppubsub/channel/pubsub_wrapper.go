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

package channel

import (
	"context"
	"errors"

	"google.golang.org/api/option"

	"golang.org/x/oauth2/google"

	"cloud.google.com/go/pubsub"
)

// This file exists so that we can unit test failures with the PubSub client.

// pubSubClientCreator creates a pubSubClient.
type pubSubClientCreator func(ctx context.Context, creds *google.Credentials, googleCloudProject string) (pubSubClient, error)

// gcpPubSubClientCreator creates a real GCP PubSub client. It should always be used, except during
// unit tests.
func gcpPubSubClientCreator(ctx context.Context, creds *google.Credentials, googleCloudProject string) (pubSubClient, error) {
	// Auth to GCP is handled by having the GOOGLE_APPLICATION_CREDENTIALS environment variable
	// pointing at a credential file.
	psc, err := pubsub.NewClient(ctx, googleCloudProject, option.WithCredentials(creds))
	if err != nil {
		return nil, err
	}
	return &realGcpPubSubClient{
		client: psc,
	}, nil
}

// pubSubClient is the set of methods we use on pubsub.Client.
type pubSubClient interface {
	SubscriptionInProject(id, projectId string) pubSubSubscription
	CreateSubscription(ctx context.Context, id string, topic pubSubTopic) (pubSubSubscription, error)
	Topic(id string) pubSubTopic
	CreateTopic(ctx context.Context, id string) (pubSubTopic, error)
}

// realGcpPubSubClient is the client that will be used everywhere except unit tests. Verify that it
// satisfies the interface.
var _ pubSubClient = &realGcpPubSubClient{}

// realGcpPubSubClient wraps a real GCP PubSub client, so that it matches the pubSubClient
// interface. It is needed because the real SubscriptionInProject returns a struct and does not
// implicitly match gcpPubSubClient, which returns an interface.
type realGcpPubSubClient struct {
	client *pubsub.Client
}

func (c *realGcpPubSubClient) SubscriptionInProject(id, projectId string) pubSubSubscription {
	return c.client.SubscriptionInProject(id, projectId)
}

func (c *realGcpPubSubClient) CreateSubscription(ctx context.Context, id string, topic pubSubTopic) (pubSubSubscription, error) {
	// We are using the real client, so this better be a real *pubsub.Topic...
	realTopic, ok := topic.(*pubsub.Topic)
	if !ok {
		return nil, errors.New("topic was not a real *pubsub.Topic")
	}
	cfg := pubsub.SubscriptionConfig{
		Topic: realTopic,
	}
	return c.client.CreateSubscription(ctx, id, cfg)
}

func (c *realGcpPubSubClient) Topic(id string) pubSubTopic {
	return c.client.Topic(id)
}

func (c *realGcpPubSubClient) CreateTopic(ctx context.Context, id string) (pubSubTopic, error) {
	return c.client.CreateTopic(ctx, id)
}

// pubSubSubscription is the set of methods we use on pubsub.Subscription. It exists to make
// pubSubClient unit testable.
type pubSubSubscription interface {
	Exists(ctx context.Context) (bool, error)
	ID() string
	Delete(ctx context.Context) error
}

// pubsub.Subscription is the real pubSubSubscription that is used everywhere except unit tests.
// Verify that it satisfies the interface.
var _ pubSubSubscription = &pubsub.Subscription{}

type pubSubTopic interface {
	Exists(ctx context.Context) (bool, error)
	ID() string
	Delete(ctx context.Context) error
}

var _ pubSubTopic = &pubsub.Topic{}
