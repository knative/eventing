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

const (
	allowAny             = "allow_any"
	allowRegisteredTypes = "allow_registered"
	autoCreate           = "auto_create"
)

var (
	// Only allow alphanumeric, '-' or '.'.
	validChars = regexp.MustCompile(`[^-\.a-z0-9]+`)
	// EventType not found error.
	notFound = k8serrors.NewNotFound(eventingv1alpha1.Resource("eventtype"), "")
)

type IngressPolicy interface {
	AllowEvent(event *cloudevents.Event, namespace string) bool
}

type Any struct{}

type Registered struct {
	logger *zap.SugaredLogger
	client client.Client
}

type AutoCreate struct {
	logger *zap.SugaredLogger
	client client.Client
}

func NewIngressPolicy(logger *zap.SugaredLogger, client client.Client, policy string) IngressPolicy {
	return newIngressPolicy(logger, client, policy)
}

func newIngressPolicy(logger *zap.SugaredLogger, client client.Client, policy string) IngressPolicy {
	switch policy {
	case allowRegisteredTypes:
		return &Registered{
			logger: logger,
			client: client,
		}
	case autoCreate:
		return &AutoCreate{
			logger: logger,
			client: client,
		}
	case allowAny:
		return &Any{}
	default:
		return &Any{}
	}
}

func (p *Any) AllowEvent(event *cloudevents.Event, namespace string) bool {
	return true
}

func (p *Registered) AllowEvent(event *cloudevents.Event, namespace string) bool {
	_, err := getEventType(p.client, event, namespace)
	if k8serrors.IsNotFound(err) {
		p.logger.Warnf("EventType not found, rejecting spec.type %q, %v", event.Type(), err)
		return false
	} else if err != nil {
		p.logger.Errorf("Error retrieving EventType, rejecting spec.type %q, %v", event.Type(), err)
		return false
	}
	return true
}

func (p *AutoCreate) AllowEvent(event *cloudevents.Event, namespace string) bool {
	_, err := getEventType(p.client, event, namespace)
	if k8serrors.IsNotFound(err) {
		p.logger.Infof("EventType not found, creating spec.type %q", event.Type())
		eventType := makeEventType(event, namespace)
		err := p.client.Create(context.TODO(), eventType)
		if err != nil {
			p.logger.Errorf("Error creating EventType, spec.type %q, %v", event.Type(), err)
		}
	} else if err != nil {
		p.logger.Errorf("Error retrieving EventType, spec.type %q, %v", event.Type(), err)
	}
	return true
}

func getEventType(c client.Client, event *cloudevents.Event, namespace string) (*eventingv1alpha1.EventType, error) {
	opts := &client.ListOptions{
		Namespace: namespace,
		// TODO add label selector on broker name.
		// Set Raw because if we need to get more than one page, then we will put the continue token
		// into opts.Raw.Continue.
		Raw: &metav1.ListOptions{},
	}

	ctx := context.TODO()

	for {
		el := &eventingv1alpha1.EventTypeList{}
		err := c.List(ctx, opts, el)
		if err != nil {
			return nil, err
		}
		for _, e := range el.Items {
			// TODO what about source?
			if e.Spec.Type == event.Type() && e.Spec.Schema == event.SchemaURL() {
				return &e, nil
			}

		}
		if el.Continue != "" {
			opts.Raw.Continue = el.Continue
		} else {
			return nil, notFound
		}
	}
}

// makeEventType generates, but does not create an EventType from the given cloudevents.Event.
func makeEventType(event *cloudevents.Event, namespace string) *eventingv1alpha1.EventType {
	cloudEventType := event.Type()
	return &eventingv1alpha1.EventType{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", toValidIdentifier(cloudEventType)),
			Namespace:    namespace,
			// TODO add broker label
			// Waiting on https://github.com/knative/eventing/pull/937
			// which passes the broker name as an env variable.
		},
		Spec: eventingv1alpha1.EventTypeSpec{
			Type:   cloudEventType,
			Source: event.Source(),
			Schema: event.SchemaURL(),
			// Waiting on https://github.com/knative/eventing/pull/937
			Broker: "",
		},
	}
}

// TODO some utility to also be used from eventing-sources.
func toValidIdentifier(cloudEventType string) string {
	if msgs := validation.IsDNS1123Subdomain(cloudEventType); len(msgs) != 0 {
		// If it is not a valid DNS1123 name, make it a valid one.
		// TODO take care of size < 63, and starting and end indexes should be alphanumeric.
		cloudEventType = strings.ToLower(cloudEventType)
		cloudEventType = validChars.ReplaceAllString(cloudEventType, "")
	}
	return cloudEventType
}
