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

package eventtype

import (
	"context"
	"crypto/md5" //nolint:gosec
	"fmt"
	"time"

	"github.com/cloudevents/sdk-go/v2/event"
	"go.uber.org/zap"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"knative.dev/eventing/pkg/apis/eventing/v1beta2"
	"knative.dev/eventing/pkg/apis/feature"
	eventingv1beta2 "knative.dev/eventing/pkg/client/clientset/versioned/typed/eventing/v1beta2"
	v1beta22 "knative.dev/eventing/pkg/client/listers/eventing/v1beta2"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

type EventTypeAutoHandler struct {
	EventTypeLister v1beta22.EventTypeLister
	EventingClient  eventingv1beta2.EventingV1beta2Interface
	FeatureStore    *feature.Store
	Logger          *zap.Logger
}

// generateEventTypeName is a pseudo unique name for EvenType object based on the input params
func generateEventTypeName(name, namespace, eventType, eventSource string) string {
	suffixParts := eventType + eventSource + namespace + name
	suffix := md5.Sum([]byte(suffixParts)) //nolint:gosec
	return utils.ToDNS1123Subdomain(fmt.Sprintf("%s-%s-%x", "et", name, suffix))
}

// AutoCreateEventType creates EventType object based on processed event's types from addressable KReference objects
func (h *EventTypeAutoHandler) AutoCreateEventType(ctx context.Context, event *event.Event, addressable *duckv1.KReference, ownerUID types.UID) {
	// Feature flag gate
	if !h.FeatureStore.IsEnabled(feature.EvenTypeAutoCreate) {
		h.Logger.Debug("Event Type auto creation is disabled")
		return
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second*30)
	go func() {
		h.Logger.Debug("Event Types auto creation is enabled")

		eventTypeName := generateEventTypeName(addressable.Name, addressable.Namespace, event.Type(), event.Source())

		exists, err := h.EventTypeLister.EventTypes(addressable.Namespace).Get(eventTypeName)
		if err != nil && !apierrs.IsNotFound(err) {
			h.Logger.Error("Failed to retrieve Even Type", zap.Error(err))
			cancel()
			return
		}
		if exists != nil {
			cancel()
			return
		}

		source, _ := apis.ParseURL(event.Source())
		schema, _ := apis.ParseURL(event.DataSchema())

		et := &v1beta2.EventType{
			ObjectMeta: metav1.ObjectMeta{
				Name:      eventTypeName,
				Namespace: addressable.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: addressable.APIVersion,
						Kind:       addressable.Kind,
						Name:       addressable.Name,
						UID:        ownerUID,
					},
				},
			},
			Spec: v1beta2.EventTypeSpec{
				Type:        event.Type(),
				Source:      source,
				Schema:      schema,
				SchemaData:  event.DataSchema(),
				Reference:   addressable,
				Description: "Event Type auto-created by controller",
			},
		}

		_, err = h.EventingClient.EventTypes(et.Namespace).Create(ctx, et, metav1.CreateOptions{})
		if err != nil && !apierrs.IsAlreadyExists(err) {
			h.Logger.Error("Failed to create Event Type", zap.Error(err))
			cancel()
			return
		}
		cancel()
	}()
}
