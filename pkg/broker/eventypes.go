/*
Copyright 2023 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package broker

import (
	"context"
	"encoding/base64"
	"github.com/cloudevents/sdk-go/v2/event"
	"go.uber.org/zap"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1beta2"
	"knative.dev/eventing/pkg/apis/feature"
	eventingv1beta2 "knative.dev/eventing/pkg/client/clientset/versioned/typed/eventing/v1beta2"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1"
	v1beta22 "knative.dev/eventing/pkg/client/listers/eventing/v1beta2"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/apis"
	v1 "knative.dev/pkg/apis/duck/v1"
)

type EventtypeAutoHandler struct {
	BrokerLister    eventinglisters.BrokerLister
	EventTypeLister v1beta22.EventTypeLister
	EventingClient  eventingv1beta2.EventingV1beta2Interface
	FeatureStore    *feature.Store
	Logger          *zap.Logger
}

func (h *EventtypeAutoHandler) getBroker(name, namespace string) (*eventingv1.Broker, error) {
	broker, err := h.BrokerLister.Brokers(namespace).Get(name)
	if err != nil {
		h.Logger.Warn("Broker getter failed")
		return nil, err
	}
	return broker, nil
}

func (h *EventtypeAutoHandler) AutoCreateEventType(ctx context.Context, event *event.Event, broker types.NamespacedName) error {
	// Feature flag gate
	if !h.FeatureStore.IsEnabled(feature.EvenTypeAutoCreate) {
		h.Logger.Debug("Event Type auto creation is disabled")
		return nil
	}
	h.Logger.Debug("Event Types auto creation is enabled")

	//TODO: Is the name unique enough?
	eventTypeName := utils.ToDNS1123Subdomain("et-" + broker.Name + "-" + base64.StdEncoding.EncodeToString([]byte(broker.Name + broker.Namespace + event.Type()))[:10])

	exists, err := h.EventTypeLister.EventTypes(broker.Namespace).Get(eventTypeName)
	if err != nil && !apierrs.IsNotFound(err) {
		h.Logger.Error("Failed to retrieve Even Type", zap.Error(err))
		return err
	}
	if exists != nil {
		return nil
	}

	source, _ := apis.ParseURL(event.Source())
	schema, _ := apis.ParseURL(event.DataSchema())

	b, err := h.getBroker(broker.Name, broker.Namespace)
	if err != nil {
		return err
	}
	et := &v1beta2.EventType{
		ObjectMeta: metav1.ObjectMeta{
			Name:      eventTypeName,
			Namespace: broker.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: b.APIVersion,
					Kind:       b.Kind,
					Name:       b.Name,
					UID:        b.GetUID(),
				},
			},
		},
		Spec: v1beta2.EventTypeSpec{
			Type:   event.Type(),
			Source: source,
			Schema: schema,
			//TODO: should we try to capture?
			//SchemaData: event.DataSchema(),
			Reference: &v1.KReference{
				Kind:       b.Kind,
				Name:       b.Name,
				Namespace:  b.Namespace,
				APIVersion: b.APIVersion,
				Address:    b.Spec.Config.Address,
			},
			Description: "Event Type auto-created by controller",
		},
	}

	_, err = h.EventingClient.EventTypes(et.Namespace).Create(ctx, et, metav1.CreateOptions{})
	if err != nil && !apierrs.IsAlreadyExists(err) {
		h.Logger.Error("Failed to create Event Type", zap.Error(err))
		return err
	}
	return nil
}
