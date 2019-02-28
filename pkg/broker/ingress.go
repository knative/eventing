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

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners"
	"go.uber.org/zap"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	allowAny             = "allow_any"
	allowRegisteredTypes = "allow_registered"

	// This header is for cloudevents spec 0.1.
	// In the 0.2 version the HTTP header is called 'ce-type'.
	// TODO use cloudevents sdk for doing encoding/decoding of events.
	eventType = "Ce-Eventtype"
)

type IngressPolicy interface {
	AllowMessage(namespace string, message *provisioners.Message) bool
}

type AllowAnyPolicy struct{}

type AllowRegisteredPolicy struct {
	logger *zap.SugaredLogger
	client client.Client
}

func NewIngressPolicy(logger *zap.SugaredLogger, client client.Client, policy string) IngressPolicy {
	return newIngressPolicy(logger, client, policy)
}

func newIngressPolicy(logger *zap.SugaredLogger, client client.Client, policy string) IngressPolicy {
	switch policy {
	case allowRegisteredTypes:
		return &AllowRegisteredPolicy{
			logger: logger,
			client: client,
		}
	case allowAny:
		return &AllowAnyPolicy{}
	default:
		return &AllowAnyPolicy{}
	}
}

func (policy *AllowAnyPolicy) AllowMessage(namespace string, message *provisioners.Message) bool {
	return true
}

func (policy *AllowRegisteredPolicy) AllowMessage(namespace string, message *provisioners.Message) bool {
	et := &eventingv1alpha1.EventType{}
	name := message.Headers[eventType]

	err := policy.client.Get(context.TODO(),
		types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		},
		et)

	if k8serrors.IsNotFound(err) {
		policy.logger.Warnf("EventType not found: %q", name)
		return false
	} else if err != nil {
		policy.logger.Errorf("Error getting EventType: %q, %v", name, err)
		return false
	}
	return true
}
