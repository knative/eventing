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
	"strings"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"go.uber.org/zap"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Policy is a helper struct to determine if the CloudEvents received pass the Broker's policy.
type Policy struct {
	logger    *zap.SugaredLogger
	client    client.Client
	namespace string
	broker    string
	spec      *eventingv1alpha1.BrokerPolicySpec
}

// NewPolicy creates a Policy for a particular Broker.
func NewPolicy(logger *zap.Logger, client client.Client, allowAny bool, namespace, broker string) *Policy {
	return &Policy{
		logger:    logger.Sugar(),
		client:    client,
		namespace: namespace,
		broker:    broker,
		spec: &eventingv1alpha1.BrokerPolicySpec{
			AllowAny: allowAny,
		},
	}
}

// AllowEvent filters events based on the configured policy.
func (p *Policy) AllowEvent(ctx context.Context, event cloudevents.Event) bool {
	// 1. If allowAny is set to true, then all events are allowed to enter the mesh.
	// 2. If allowAny is set to false, then the event is only accepted if it's already in the Broker's registry.
	if p.spec.AllowAny {
		p.logger.Infof("EventType (type:%s, source:%s, schema:%s) received, Accept", event.Type(), event.Source(), event.SchemaURL())
		return true
	}
	return p.isRegistered(ctx, event)
}

// isRegistered returns whether the EventType corresponding to the CloudEvent 'event' is available in the Registry.
// Use with caution as this method can affect the data plane throughput.
func (p *Policy) isRegistered(ctx context.Context, event cloudevents.Event) bool {
	_, err := p.getEventType(ctx, event)
	if apierrs.IsNotFound(err) {
		p.logger.Debugf("EventType (type:%s, source:%s, schema:%s) not found, Reject", event.Type(), event.Source(), event.SchemaURL())
		return false
	} else if err != nil {
		p.logger.Errorf("Error retrieving EventType (type:%s, source:%s, schema:%s), Reject: %v", event.Type(), event.Source(), event.SchemaURL(), err)
		return false
	}
	p.logger.Debugf("EventType (type:%s, source:%s, schema:%s) is registered, Accept", event.Type(), event.Source(), event.SchemaURL())
	return true
}

// getEventType retrieves the EventType from the Registry for the given CloudEvent 'event'.
// If it is not found, it returns an error.
func (p *Policy) getEventType(ctx context.Context, event cloudevents.Event) (*eventingv1alpha1.EventType, error) {
	opts := &client.ListOptions{
		Namespace: p.namespace,
		// Set Raw because if we need to get more than one page, then we will put the continue token
		// into opts.Raw.Continue.
		// TODO filter by Broker label.
		Raw: &metav1.ListOptions{},
	}

	for {
		etl := &eventingv1alpha1.EventTypeList{}
		err := p.client.List(ctx, opts, etl)
		if err != nil {
			return nil, err
		}
		for _, et := range etl.Items {
			if et.Spec.Broker == p.broker {
				// Matching on type, source, and schemaURL.
				// Note that if we the CloudEvent comes with a very specific source (i.e., without the split of
				// source and subject proposed in v0.3), the EventType most probably won't be there.
				if strings.EqualFold(et.Spec.Type, event.Type()) && strings.EqualFold(et.Spec.Source, event.Source()) && strings.EqualFold(et.Spec.Schema, event.SchemaURL()) {
					return &et, nil
				}
			}
		}
		if etl.Continue != "" {
			opts.Raw.Continue = etl.Continue
		} else {
			return nil, apierrs.NewNotFound(eventingv1alpha1.Resource("eventtype"), "")
		}
	}
}
