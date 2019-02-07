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

package fakepubsub

import (
	"context"

	"golang.org/x/oauth2/google"

	"github.com/knative/eventing/contrib/gcppubsub/pkg/util"

	"cloud.google.com/go/pubsub"
)

type CreatorData struct {
	ClientCreateErr error
	ClientData      ClientData
}

func Creator(value interface{}) util.PubSubClientCreator {
	var data CreatorData
	var ok bool
	if data, ok = value.(CreatorData); !ok {
		data = CreatorData{}
	}
	if data.ClientCreateErr != nil {
		return func(_ context.Context, _ *google.Credentials, _ string) (util.PubSubClient, error) {
			return nil, data.ClientCreateErr
		}
	}
	return func(_ context.Context, credentials *google.Credentials, _ string) (util.PubSubClient, error) {
		return &Client{
			Data: data.ClientData,
		}, nil
	}
}

type ClientData struct {
	SubscriptionData SubscriptionData
	CreateSubErr     error
	TopicData        TopicData
	CreateTopicErr   error
}

type Client struct {
	Data ClientData
}

var _ util.PubSubClient = &Client{}

func (c *Client) SubscriptionInProject(id, projectId string) util.PubSubSubscription {
	return &Subscription{
		Data: c.Data.SubscriptionData,
	}
}

func (c *Client) CreateSubscription(ctx context.Context, id string, topic util.PubSubTopic) (util.PubSubSubscription, error) {
	if c.Data.CreateSubErr != nil {
		return nil, c.Data.CreateSubErr
	}
	return &Subscription{
		Data: c.Data.SubscriptionData,
	}, nil
}

func (c *Client) Topic(id string) util.PubSubTopic {
	return &Topic{
		Data: c.Data.TopicData,
	}
}

func (c *Client) CreateTopic(ctx context.Context, id string) (util.PubSubTopic, error) {
	if c.Data.CreateTopicErr != nil {
		return nil, c.Data.CreateTopicErr
	}
	return c.Topic(id), nil
}

type SubscriptionData struct {
	Exists     bool
	ExistsErr  error
	DeleteErr  error
	ReceiveErr error

	ReceiveFunc func(context.Context, util.PubSubMessage)
}

type Subscription struct {
	Data SubscriptionData
}

var _ util.PubSubSubscription = &Subscription{}

func (s *Subscription) Exists(ctx context.Context) (bool, error) {
	return s.Data.Exists, s.Data.ExistsErr
}

func (s *Subscription) ID() string {
	return "test-subscription-id"
}

func (s *Subscription) Delete(ctx context.Context) error {
	return s.Data.DeleteErr
}

func (s *Subscription) Receive(ctx context.Context, f func(context.Context, util.PubSubMessage)) error {
	s.Data.ReceiveFunc = f
	return s.Data.ReceiveErr
}

type TopicData struct {
	Exists    bool
	ExistsErr error

	DeleteErr error
	Publish   PublishResultData

	Stop bool
}

type Topic struct {
	Data TopicData
}

var _ util.PubSubTopic = &Topic{}

func (t *Topic) Exists(ctx context.Context) (bool, error) {
	return t.Data.Exists, t.Data.ExistsErr
}

func (t *Topic) ID() string {
	return "test-topic-ID"
}

func (t *Topic) Delete(ctx context.Context) error {
	return t.Data.DeleteErr
}

func (t *Topic) Publish(ctx context.Context, msg *pubsub.Message) util.PubSubPublishResult {
	return &PublishResult{
		Data: t.Data.Publish,
	}
}

func (t *Topic) Stop() {
	t.Data.Stop = true
}

type PublishResultData struct {
	ID    string
	Err   error
	Ready <-chan struct{}
}

type PublishResult struct {
	Data PublishResultData
}

var _ util.PubSubPublishResult = &PublishResult{}

func (r *PublishResult) Ready() <-chan struct{} {
	return r.Data.Ready
}

func (r *PublishResult) Get(ctx context.Context) (serverID string, err error) {
	return r.Data.ID, r.Data.Err
}

type MessageData struct {
	Ack  bool
	Nack bool
}

type Message struct {
	MessageData MessageData
}

var _ util.PubSubMessage = &Message{}

func (m *Message) ID() string {
	return "test-message-id"
}

func (m *Message) Data() []byte {
	return []byte("test-message-data")
}

func (m *Message) Attributes() map[string]string {
	return map[string]string{
		"test": "attributes",
	}
}

func (m *Message) Ack() {
	m.MessageData.Ack = true
}

func (m *Message) Nack() {
	m.MessageData.Nack = true
}
