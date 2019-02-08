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
	"fmt"
	"reflect"
	"unsafe"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/selection"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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
	// This conversion to selector should be done only once, possibly upon creation of the trigger
	// in case the filters are immutable. All validations should be performed then.
	selector, err := r.buildSelector(t)
	if err != nil {
		r.logger.Error("Invalid selector for trigger spec", zap.Any("triggerSpec", t))
		return false
	}
	matched := selector.Matches(labels.Set(m.Headers))
	if !matched {
		r.logger.Debug("Selector did not match message headers", zap.String("selector", selector.String()), zap.Any("headers", m.Headers))
	}
	return matched
}

func (r *Receiver) buildSelector(ts *eventingv1alpha1.TriggerSpec) (labels.Selector, error) {
	// Avoid validation of keys and values, otherwise we cannot use LabelSelectors.
	// Eventually, we will need to create our own Selector implementation with our own Requirement struct.
	selector := labels.SelectorFromValidatedSet(labels.Set(ts.Filter.Headers))
	for _, expr := range ts.Filter.HeaderExpressions {
		var op selection.Operator
		switch expr.Operator {
		case v1.LabelSelectorOpIn:
			op = selection.In
		case v1.LabelSelectorOpNotIn:
			op = selection.NotIn
		case v1.LabelSelectorOpExists:
			op = selection.Exists
		case v1.LabelSelectorOpDoesNotExist:
			op = selection.DoesNotExist
		default:
			return nil, fmt.Errorf("%q is not a valid filter selector operator", expr.Operator)
		}
		// Hack to set Requirement's unexposed fields to easily support expressions using k8s LabelSelectors.
		// We should change this once we agree on a filter API.
		r := labels.Requirement{}
		rr := reflect.ValueOf(&r).Elem()
		// Setting key
		rrKey := rr.FieldByName("key")
		rrKey = reflect.NewAt(rrKey.Type(), unsafe.Pointer(rrKey.UnsafeAddr())).Elem()
		rrKey.SetString(expr.Key)
		// Setting operator
		rrOperator := rr.FieldByName("operator")
		rrOperator = reflect.NewAt(rrOperator.Type(), unsafe.Pointer(rrOperator.UnsafeAddr())).Elem()
		rrOperator.Set(reflect.ValueOf(op))
		// Setting strValues
		rrStrValues := rr.FieldByName("strValues")
		rrStrValues = reflect.NewAt(rrStrValues.Type(), unsafe.Pointer(rrStrValues.UnsafeAddr())).Elem()
		rrStrValues.Set(reflect.ValueOf(expr.Values))
		selector = selector.Add(r)
	}
	return selector, nil
}
