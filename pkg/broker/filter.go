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

package broker

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/labels"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	Any = "Any"
)

// Receiver parses Cloud Events and sends them to GCP PubSub.
type Receiver struct {
	logger *zap.Logger
	client client.Client

	dispatcher provisioners.Dispatcher
}

// New creates a new Receiver and its associated MessageReceiver. The caller is responsible for
// Start()ing the returned MessageReceiver.
func New(logger *zap.Logger, client client.Client) (*Receiver, manager.Runnable) {
	r := &Receiver{
		logger:     logger,
		client:     client,
		dispatcher: provisioners.NewMessageDispatcher(logger.Sugar()),
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

	t, err := r.getTrigger(ctx, channel)
	if err != nil {
		r.logger.Info("Unable to get the Trigger", zap.Error(err), zap.Any("channelRef", channel))
		return err
	}

	subscriberURI := t.Status.SubscriberURI
	if subscriberURI == "" {
		r.logger.Error("Unable to read subscriberURI")
		return errors.New("unable to read subscriberURI")
	}

	if !r.shouldSendMessage(&t.Spec, message) {
		r.logger.Debug("Message did not pass filter")
		return nil
	}

	err = r.dispatcher.DispatchMessage(message, subscriberURI, "", provisioners.DispatchDefaults{})
	if err != nil {
		r.logger.Info("Failed to dispatch message", zap.Error(err))
		return err
	}
	r.logger.Debug("Successfully sent message")
	return nil
}

func (r *Receiver) getTrigger(ctx context.Context, ref provisioners.ChannelReference) (*eventingv1alpha1.Trigger, error) {
	t := &eventingv1alpha1.Trigger{}
	err := r.client.Get(ctx,
		types.NamespacedName{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		},
		t)
	return t, err
}

func (r *Receiver) shouldSendMessage(t *eventingv1alpha1.TriggerSpec, m *provisioners.Message) bool {
	// TODO, this conversion to selector should be done only once, possibly upon creation of the trigger
	// in case the filters are immutable
	selector, err := v1.LabelSelectorAsSelector(t.Filters)
	if err != nil {
		r.logger.Error("Invalid label selector for filter", zap.Error(err))
		return false
	}
	l := labels.Set(m.Headers)
	return selector.Matches(l)
}
