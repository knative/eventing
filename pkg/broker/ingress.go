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
	"regexp"
	"strings"

	"go.uber.org/zap"

	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// Only allow alphanumeric, '-' or '.'.
	validChars = regexp.MustCompile(`[^-\.a-z0-9]+`)
	// EventType not found error.
	notFound = k8serrors.NewNotFound(eventingv1alpha1.Resource("eventtype"), "")
)

type IngressPolicy struct {
	logger    *zap.SugaredLogger
	client    client.Client
	namespace string
	broker    string
	spec      *eventingv1alpha1.IngressPolicySpec
	async     bool
}

func NewPolicy(logger *zap.Logger, client client.Client, spec *eventingv1alpha1.IngressPolicySpec, namespace, broker string, async bool) *IngressPolicy {
	return &IngressPolicy{
		logger:    logger.Sugar(),
		client:    client,
		namespace: namespace,
		broker:    broker,
		spec:      spec,
		async:     async,
	}
}

func (p *IngressPolicy) AllowEvent(ctx context.Context, event cloudevents.Event) bool {
	if p.spec.AutoAdd {
		return p.autoAdd(ctx, event)
	}
	if !p.spec.AllowAny {
		return p.isRegistered(ctx, event)
	}
	return true
}

func (p *IngressPolicy) autoAdd(ctx context.Context, event cloudevents.Event) bool {
	addFunc := func(ctx context.Context) {
		_, err := p.getEventType(ctx, event)
		if k8serrors.IsNotFound(err) {
			p.logger.Infof("EventType %q not found: Adding", event.Type())
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

func (p *IngressPolicy) isRegistered(ctx context.Context, event cloudevents.Event) bool {
	_, err := p.getEventType(ctx, event)
	if k8serrors.IsNotFound(err) {
		p.logger.Warnf("EventType %q not found: Reject", event.Type())
		return false
	} else if err != nil {
		p.logger.Errorf("Error retrieving EventType %q: Reject, %v", event.Type(), err)
		return false
	}
	return true
}

func (p *IngressPolicy) getEventType(ctx context.Context, event cloudevents.Event) (*eventingv1alpha1.EventType, error) {
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
				// Matching on type and schemaURL, although this does not uniquely identify the EventType.
				// If we match on source, then we will never find the EventType.
				if et.Spec.Type == event.Type() && et.Spec.Schema == event.SchemaURL() {
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
	cloudEventType := event.Type()
	return &eventingv1alpha1.EventType{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", toDNS1123Subdomain(cloudEventType)),
			Namespace:    p.namespace,
		},
		Spec: eventingv1alpha1.EventTypeSpec{
			Type:   cloudEventType,
			Source: event.Source(),
			Schema: event.SchemaURL(),
			Broker: p.broker,
		},
	}
}

func toDNS1123Subdomain(cloudEventType string) string {
	// If it is not a valid DNS1123 subdomain, make it a valid one.
	if msgs := validation.IsDNS1123Subdomain(cloudEventType); len(msgs) != 0 {
		// If the length exceeds the max, cut it and leave some room for the generated UUID.
		if len(cloudEventType) > validation.DNS1123SubdomainMaxLength {
			cloudEventType = cloudEventType[:validation.DNS1123SubdomainMaxLength-10]
		}
		cloudEventType = strings.ToLower(cloudEventType)
		cloudEventType = validChars.ReplaceAllString(cloudEventType, "")
		// Only start/end with alphanumeric.
		cloudEventType = strings.Trim(cloudEventType, "-.")
	}
	return cloudEventType
}
