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

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Receiver parses Cloud Events and sends them to the channel.
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
	return provisioners.NewMessageReceiver(r.sendEvent, r.logger.Sugar())
}

// sendEvent sends a message to the Channel.
func (r *Receiver) sendEvent(channel provisioners.ChannelReference, message *provisioners.Message) error {
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
	// Sadly this doesn't work well because we do not yet have
	// https://github.com/kubernetes-sigs/controller-runtime/pull/136, so controller runtime watches
	// all Triggers, not just those in this namespace. And it doesn't have the RBAC (by default) for
	// that to work.
	t := &eventingv1alpha1.Trigger{}
	err := r.client.Get(ctx,
		types.NamespacedName{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		},
		t)
	return t, err
}

func (r *Receiver) shouldSendMessage(ts *eventingv1alpha1.TriggerSpec, m *provisioners.Message) bool {
	if ts.Filter == nil || ts.Filter.SourceAndType == nil {
		r.logger.Error("No filter specified")
		return false
	}
	filterType := ts.Filter.SourceAndType.Type
	if filterType != eventingv1alpha1.TriggerAnyFilter && filterType != m.Headers["Ce-Eventtype"] {
		r.logger.Debug("Wrong type", zap.String("trigger.spec.filter.exactMatch.type", filterType), zap.String("message.type", m.Headers["Ce-Eventtype"]))
		return false
	}
	filterSource := ts.Filter.SourceAndType.Source
	if filterSource != eventingv1alpha1.TriggerAnyFilter && filterSource != m.Headers["Ce-Source"] {
		r.logger.Debug("Wrong source", zap.String("trigger.spec.filter.exactMatch.source", filterSource), zap.String("message.source", m.Headers["Ce-Source"]))
		return false
	}
	return true
}
