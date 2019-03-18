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

package util

import (
	"context"
	"errors"

	"cloud.google.com/go/pubsub"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

// This file exists so that we can unit test failures with the PubSub client.

// PubSubClientCreator creates a pubSubClient.
type PubSubClientCreator func(ctx context.Context, creds *google.Credentials, googleCloudProject string) (PubSubClient, error)

// GcpPubSubClientCreator creates a real GCP PubSub client. It should always be used, except during
// unit tests.
func GcpPubSubClientCreator(ctx context.Context, creds *google.Credentials, googleCloudProject string) (PubSubClient, error) {
	psc, err := pubsub.NewClient(ctx, googleCloudProject, option.WithCredentials(creds))
	if err != nil {
		return nil, err
	}
	return &realGcpPubSubClient{
		client: psc,
	}, nil
}

// PubSubClient is the set of methods we use on pubsub.Client. See pubsub.Client for documentation
// of the functions.
type PubSubClient interface {
	SubscriptionInProject(id, projectId string) PubSubSubscription
	CreateSubscription(ctx context.Context, id string, topic PubSubTopic) (PubSubSubscription, error)
	Topic(id string) PubSubTopic
	CreateTopic(ctx context.Context, id string) (PubSubTopic, error)
}

// realGcpPubSubClient is the client that will be used everywhere except unit tests. Verify that it
// satisfies the interface.
var _ PubSubClient = &realGcpPubSubClient{}

// realGcpPubSubClient wraps a real GCP PubSub client, so that it matches the PubSubClient
// interface. It is needed because the real SubscriptionInProject returns a struct and does not
// implicitly match gcpPubSubClient, which returns an interface.
type realGcpPubSubClient struct {
	client *pubsub.Client
}

func (c *realGcpPubSubClient) SubscriptionInProject(id, projectId string) PubSubSubscription {
	return &realGcpPubSubSubscription{
		sub: c.client.SubscriptionInProject(id, projectId),
	}
}

func (c *realGcpPubSubClient) CreateSubscription(ctx context.Context, id string, topic PubSubTopic) (PubSubSubscription, error) {
	// We are using the real client, so this better be a real *pubsub.Topic...
	realTopic, ok := topic.(*realGcpPubSubTopic)
	if !ok {
		return nil, errors.New("topic was not a *realGcpPubSubTopic")
	}
	cfg := pubsub.SubscriptionConfig{
		Topic: realTopic.topic,
	}
	sub, err := c.client.CreateSubscription(ctx, id, cfg)
	return &realGcpPubSubSubscription{sub: sub}, err
}

func (c *realGcpPubSubClient) Topic(id string) PubSubTopic {
	return &realGcpPubSubTopic{topic: c.client.Topic(id)}
}

func (c *realGcpPubSubClient) CreateTopic(ctx context.Context, id string) (PubSubTopic, error) {
	topic, err := c.client.CreateTopic(ctx, id)
	return &realGcpPubSubTopic{topic: topic}, err
}

// PubSubSubscription is the set of methods we use on pubsub.Subscription. It exists to make
// PubSubClient unit testable. See pubsub.Subscription for documentation of the functions.
type PubSubSubscription interface {
	Exists(ctx context.Context) (bool, error)
	ID() string
	Delete(ctx context.Context) error
	Receive(ctx context.Context, f func(context.Context, PubSubMessage)) error
}

// realGcpPubSubSubscription wraps a real GCP PubSub Subscription, so that it matches the
// PubSubSubscription interface.
type realGcpPubSubSubscription struct {
	sub *pubsub.Subscription
}

// realGcpPubSubSubscription is the real PubSubSubscription that is used everywhere except unit
// tests. Verify that it satisfies the interface.
var _ PubSubSubscription = &realGcpPubSubSubscription{}

func (s *realGcpPubSubSubscription) Exists(ctx context.Context) (bool, error) {
	return s.sub.Exists(ctx)
}

func (s *realGcpPubSubSubscription) ID() string {
	return s.sub.ID()
}

func (s *realGcpPubSubSubscription) Delete(ctx context.Context) error {
	return s.sub.Delete(ctx)
}

func (s *realGcpPubSubSubscription) Receive(ctx context.Context, f func(context.Context, PubSubMessage)) error {
	fWrapper := func(ctx context.Context, msg *pubsub.Message) {
		f(ctx, &realGcpPubSubMessage{msg: msg})
	}
	return s.sub.Receive(ctx, fWrapper)
}

// PubSubTopic is the set of methods we use on pubsub.Topic. It exists to make PubSubClient unit
// testable. See pubsub.Topic for documentation of the functions.
type PubSubTopic interface {
	Exists(ctx context.Context) (bool, error)
	ID() string
	Delete(ctx context.Context) error
	Publish(ctx context.Context, msg *pubsub.Message) PubSubPublishResult
	Stop()
}

// realGcpPubSubTopic wraps a real GCP PubSub Topic, so that it matches the PubSubTopic interface.
type realGcpPubSubTopic struct {
	topic *pubsub.Topic
}

// realGcpPubSubTopic is the real PubSubTopic that is used everywhere except unit tests. Verify that
// it satisfies the interface.
var _ PubSubTopic = &realGcpPubSubTopic{}

func (t *realGcpPubSubTopic) Exists(ctx context.Context) (bool, error) {
	return t.topic.Exists(ctx)
}

func (t *realGcpPubSubTopic) ID() string {
	return t.topic.ID()
}

func (t *realGcpPubSubTopic) Delete(ctx context.Context) error {
	return t.topic.Delete(ctx)
}

func (t *realGcpPubSubTopic) Publish(ctx context.Context, msg *pubsub.Message) PubSubPublishResult {
	return t.topic.Publish(ctx, msg)
}

func (t *realGcpPubSubTopic) Stop() {
	t.topic.Stop()
}

// PubSubPublishResult is the set of methods we use on pubsub.PublishResult. It exists to make
// PubSubClient unit testable. See pubsub.PublishResult for documentation of any functions.
type PubSubPublishResult interface {
	Ready() <-chan struct{}
	Get(ctx context.Context) (serverID string, err error)
}

// pubsub.PublishResult is the real PubSubPublishResult used everywhere except unit tests. Verify
// that it satisfies the interface.
var _ PubSubPublishResult = &pubsub.PublishResult{}

// PubSubMessage is the set of methods we use on pubsub.Message. It exists to make PubSubClient unit
// testable. See pubsub.Message for documentation of the functions.
type PubSubMessage interface {
	ID() string
	Data() []byte
	Attributes() map[string]string
	Ack()
	Nack()
}

// realGcpPubSubMessage wraps a real GCP PubSub Message, so that it matches the PubSubMessage
// interface.
type realGcpPubSubMessage struct {
	msg *pubsub.Message
}

// realGcpPubSubMessage is the real PubSubMessage used everywhere except unit tests. Verify that it
// satisfies the interface.
var _ PubSubMessage = &realGcpPubSubMessage{}

func (m *realGcpPubSubMessage) ID() string {
	return m.msg.ID
}

func (m *realGcpPubSubMessage) Data() []byte {
	return m.msg.Data
}

func (m *realGcpPubSubMessage) Attributes() map[string]string {
	return m.msg.Attributes
}

func (m *realGcpPubSubMessage) Ack() {
	m.msg.Ack()
}

func (m *realGcpPubSubMessage) Nack() {
	m.msg.Nack()
}
