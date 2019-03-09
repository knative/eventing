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

	"github.com/cloudevents/sdk-go/pkg/cloudevents"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	allowAny             = "allow_any"
	allowRegisteredTypes = "allow_registered"
)

type IngressPolicy interface {
	AllowEvent(namespace string, event *cloudevents.Event) bool
}

type AllowAnyPolicy struct{}

type AllowRegisteredPolicy struct {
	client client.Client
}

func NewIngressPolicy(client client.Client, policy string) IngressPolicy {
	return newIngressPolicy(client, policy)
}

func newIngressPolicy(client client.Client, policy string) IngressPolicy {
	switch policy {
	case allowRegisteredTypes:
		return &AllowRegisteredPolicy{
			client: client,
		}
	case allowAny:
		return &AllowAnyPolicy{}
	default:
		return &AllowAnyPolicy{}
	}
}

func (p *AllowAnyPolicy) AllowEvent(namespace string, event *cloudevents.Event) bool {
	return true
}

func (p *AllowRegisteredPolicy) AllowEvent(namespace string, event *cloudevents.Event) bool {
	opts := &client.ListOptions{
		Namespace: namespace,
		// Set Raw because if we need to get more than one page, then we will put the continue token
		// into opts.Raw.Continue.
		Raw: &metav1.ListOptions{},
	}

	ctx := context.TODO()

	// TODO this is very inefficient, we need to somehow select with a fieldSelector on spec.type,
	// but it is not supported.
	for {
		el := &eventingv1alpha1.EventTypeList{}
		err := p.client.List(ctx, opts, el)
		if err != nil {
			return false
		}
		for _, e := range el.Items {
			if e.Spec.Type == event.Type() {
				return true
			}
		}
		if el.Continue != "" {
			opts.Raw.Continue = el.Continue
		} else {
			return false
		}
	}
}
