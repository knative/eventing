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
	"fmt"

	"github.com/knative/eventing/pkg/utils"

	"go.uber.org/zap"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// EventType not found error.
	notFound = k8serrors.NewNotFound(eventingv1alpha1.Resource("eventtype"), "")
)

// IngressPolicy parses Cloud Events, determines if they pass the Broker's policy, and sends them downstream.
type IngressPolicy struct {
	logger    *zap.SugaredLogger
	client    client.Client
	namespace string
	broker    string
	spec      *eventingv1alpha1.IngressPolicySpec
	// This bool flag is for UT purposes only.
	async bool
}

// NewPolicy creates an IngressPolicy for a particular Broker.
func NewPolicy(logger *zap.Logger, client client.Client, spec *eventingv1alpha1.IngressPolicySpec, namespace, broker string, async bool) *IngressPolicy {
	// TODO too many args, maybe use a builder or just remove this function.
	return &IngressPolicy{
		logger:    logger.Sugar(),
		client:    client,
		namespace: namespace,
		broker:    broker,
		spec:      spec,
		async:     async,
	}
}

// AllowEvent filters events based on the configured policy.
func (p *IngressPolicy) AllowEvent(ctx context.Context, event cloudevents.Event) bool {
	// 1. If autoAdd is set to true, then the event is accepted. In case it wasn't seen before, it is added to the Broker's registry.
	// 2. If allowAny is set to false, then the event is only accepted if it's already in the Broker's registry.
	// 3. If allowAny is set to true, then all events are allowed to enter the mesh.
	if p.spec.AutoAdd {
		return p.autoAdd(ctx, event)
	}
	if !p.spec.AllowAny {
		return p.isRegistered(ctx, event)
	}
	return true
}

// autoAdd attempts to add the EventType corresponding to the CloudEvent if it's not available in the Registry.
// It always returns true.
func (p *IngressPolicy) autoAdd(ctx context.Context, event cloudevents.Event) bool {
	addFunc := func(ctx context.Context) {
		_, err := p.getEventType(ctx, event)
		if k8serrors.IsNotFound(err) {
			p.logger.Debugf("EventType %q not found: Adding", event.Type())
			eventType := p.makeEventType(event)
			err := p.client.Create(ctx, eventType)
			if err != nil {
				p.logger.Errorf("Error creating EventType %q: Accept but Not Add, %v", event.Type(), err)
			}
		} else if err != nil {
			p.logger.Errorf("Error retrieving EventType %q: Accept but Not Add, %v", event.Type(), err)
		}
	}
	if p.async {
		// TODO do this in a working queue
		// Do not use the previous context as it seems that it can be canceled before
		// this routine executes.
		go addFunc(context.TODO())
	} else {
		addFunc(ctx)
	}
	return true
}

// isRegistered returns whether the EventType corresponding to the CloudEvent is available in the Registry.
func (p *IngressPolicy) isRegistered(ctx context.Context, event cloudevents.Event) bool {
	_, err := p.getEventType(ctx, event)
	if k8serrors.IsNotFound(err) {
		p.logger.Debugf("EventType %q not found: Reject", event.Type())
		return false
	} else if err != nil {
		p.logger.Errorf("Error retrieving EventType %q: Reject, %v", event.Type(), err)
		return false
	}
	return true
}

// getEventType retrieves the EventType from the Registry for the given cloudevents.Event.
// If it is not found, it returns an error.
func (p *IngressPolicy) getEventType(ctx context.Context, event cloudevents.Event) (*eventingv1alpha1.EventType, error) {
	source := sourceOrFrom(event)

	opts := &client.ListOptions{
		Namespace: p.namespace,
		// Set Raw because if we need to get more than one page, then we will put the continue token
		// into opts.Raw.Continue.
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
				// Matching on type, schemaURL, and "source" (either the CloudEvent source or our custom extension).
				// Note that if we end up using the CloudEvent source, most probably the EventType won't be there.
				if et.Spec.Type == event.Type() && et.Spec.Source == source && et.Spec.Schema == event.SchemaURL() {
					return &et, nil
				}
			}
		}
		if etl.Continue != "" {
			opts.Raw.Continue = etl.Continue
		} else {
			return nil, notFound
		}
	}
}

// makeEventType generates, but does not create an EventType from the given cloudevents.Event.
func (p *IngressPolicy) makeEventType(event cloudevents.Event) *eventingv1alpha1.EventType {
	source := sourceOrFrom(event)
	cloudEventType := event.Type()
	return &eventingv1alpha1.EventType{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", utils.ToDNS1123Subdomain(cloudEventType)),
			Namespace:    p.namespace,
		},
		Spec: eventingv1alpha1.EventTypeSpec{
			Type:   cloudEventType,
			Source: source,
			Schema: event.SchemaURL(),
			Broker: p.broker,
		},
	}
}

// Retrieve the custom extension 'from' as opposed to CloudEvent source, if available.
// If the extension is populated, it means the event came from one of our sources.
// Note that some of our sources might not populate this, e.g., container source, etc., so we just retrieve the CloudEvent
// source.
func sourceOrFrom(event cloudevents.Event) string {
	source := event.Source()
	var from string
	err := event.ExtensionAs(extensionFrom, &from)
	if err == nil {
		source = from
	}
	return source
}
