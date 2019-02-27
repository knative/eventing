/*
 * Copyright 2019 The Knative Authors
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Receiver parses Cloud Events, determines if they pass a filter, and sends them to a subscriber.
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
	if err := r.initClient(); err != nil {
		r.logger.Warn("Failed to initialize client", zap.Error(err))
	}
	return provisioners.NewMessageReceiver(r.sendEvent, r.logger.Sugar())
}

// sendEvent sends an event to a subscriber if the trigger filter passes.
func (r *Receiver) sendEvent(trigger provisioners.ChannelReference, message *provisioners.Message) error {
	r.logger.Debug("Received message", zap.Any("triggerRef", trigger))
	ctx := context.Background()

	t, err := r.getTrigger(ctx, trigger)
	if err != nil {
		r.logger.Info("Unable to get the Trigger", zap.Error(err), zap.Any("triggerRef", trigger))
		return err
	}

	subscriberURI := t.Status.SubscriberURI
	if subscriberURI == "" {
		r.logger.Error("Unable to read subscriberURI")
		return errors.New("unable to read subscriberURI")
	}

	if !r.shouldSendMessage(&t.Spec, message) {
		r.logger.Debug("Message did not pass filter", zap.Any("triggerRef", trigger))
		return nil
	}

	err = r.dispatcher.DispatchMessage(message, subscriberURI, "", provisioners.DispatchDefaults{})
	if err != nil {
		r.logger.Info("Failed to dispatch message", zap.Error(err), zap.Any("triggerRef", trigger))
		return err
	}
	r.logger.Debug("Successfully sent message", zap.Any("triggerRef", trigger))
	return nil
}

// Initialize the client. Mainly intended to load stuff in its cache.
func (r *Receiver) initClient() error {
	// We list triggers so that we can load the client's cache. Otherwise, on receiving an event, it
	// may not find the trigger and would return an error.
	opts := &client.ListOptions{
		// Set Raw because if we need to get more than one page, then we will put the continue token
		// into opts.Raw.Continue.
		Raw: &metav1.ListOptions{},
	}
	for {
		tl := &eventingv1alpha1.TriggerList{}
		if err := r.client.List(context.TODO(), opts, tl); err != nil {
			return err
		}
		if tl.Continue != "" {
			opts.Raw.Continue = tl.Continue
		} else {
			break
		}
	}
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

// shouldSendMessage determines whether message 'm' should be sent based on the triggerSpec 'ts'.
// Currently it supports exact matching on type and/or source of events.
func (r *Receiver) shouldSendMessage(ts *eventingv1alpha1.TriggerSpec, m *provisioners.Message) bool {
	if ts.Filter == nil || ts.Filter.SourceAndType == nil {
		r.logger.Error("No filter specified")
		return false
	}
	filterType := ts.Filter.SourceAndType.Type
	if filterType != eventingv1alpha1.TriggerAnyFilter && filterType != m.Headers["Ce-Eventtype"] {
		r.logger.Debug("Wrong type", zap.String("trigger.spec.filter.sourceAndType.type", filterType), zap.String("message.type", m.Headers["Ce-Eventtype"]))
		return false
	}
	filterSource := ts.Filter.SourceAndType.Source
	if filterSource != eventingv1alpha1.TriggerAnyFilter && filterSource != m.Headers["Ce-Source"] {
		r.logger.Debug("Wrong source", zap.String("trigger.spec.filter.sourceAndType.source", filterSource), zap.String("message.source", m.Headers["Ce-Source"]))
		return false
	}
	return true
}
