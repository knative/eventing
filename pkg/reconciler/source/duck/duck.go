/*
Copyright 2020 The Knative Authors

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

package duck

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	clientset "knative.dev/eventing/pkg/client/clientset/versioned"
	listers "knative.dev/eventing/pkg/client/listers/eventing/v1beta1"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler/source/duck/resources"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

type Reconciler struct {
	// eventingClientSet allows us to configure Eventing objects
	eventingClientSet clientset.Interface

	// listers index properties about resources
	eventTypeLister listers.EventTypeLister
	crdLister       apiextensionsv1beta1.CustomResourceDefinitionLister
	sourceLister    cache.GenericLister

	gvr     schema.GroupVersionResource
	crdName string
}

// eventTypeEntry refers to an entry in the registry.knative.dev/eventTypes annotation.
type eventTypeEntry struct {
	Type        string `json:"type"`
	Schema      string `json:"schema,omitempty"`
	Description string `json:"description,omitempty"`
}

func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key")
		return nil
	}

	// Get the Source resource with this namespace/name
	runtimeObj, err := r.sourceLister.ByNamespace(namespace).Get(name)

	var ok bool
	var original *duckv1.Source
	if original, ok = runtimeObj.(*duckv1.Source); !ok {
		logging.FromContext(ctx).Error("runtime object is not convertible to Source duck type: ", zap.Any("runtimeObj", runtimeObj))
		// Avoid re-enqueuing.
		return nil
	}

	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("Source in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	orig := original.DeepCopy()
	// Reconcile this copy of the Source. We do not control the Source, so do not update status.
	return r.reconcile(ctx, orig)
}

func (r *Reconciler) reconcile(ctx context.Context, source *duckv1.Source) error {
	// Reconcile the eventTypes for this source.
	err := r.reconcileEventTypes(ctx, source)
	if err != nil {
		logging.FromContext(ctx).Error("Error reconciling event types for Source")
		return err
	}
	return nil
}

// TODO revisit most of this logic once we get rid of Broker and maybe some other bits.
//  https://github.com/knative/eventing/issues/2750.
func (r *Reconciler) reconcileEventTypes(ctx context.Context, src *duckv1.Source) error {
	current, err := r.getEventTypes(ctx, src)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get existing event types", zap.Error(err))
		return err
	}

	expected, err := r.makeEventTypes(ctx, src)
	if err != nil {
		return err
	}

	toCreate, toDelete := r.computeDiff(current, expected)

	for _, eventType := range toDelete {
		if err = r.eventingClientSet.EventingV1beta1().EventTypes(src.Namespace).Delete(eventType.Name, &metav1.DeleteOptions{}); err != nil {
			logging.FromContext(ctx).Error("Error deleting eventType", zap.Any("eventType", eventType))
			return err
		}
	}

	for _, eventType := range toCreate {
		if _, err = r.eventingClientSet.EventingV1beta1().EventTypes(src.Namespace).Create(&eventType); err != nil {
			logging.FromContext(ctx).Error("Error creating eventType", zap.Any("eventType", eventType))
			return err
		}
	}

	return nil
}

func (r *Reconciler) getEventTypes(ctx context.Context, src *duckv1.Source) ([]v1beta1.EventType, error) {
	etl, err := r.eventTypeLister.EventTypes(src.Namespace).List(labels.SelectorFromSet(resources.Labels(src.Name)))
	if err != nil {
		logging.FromContext(ctx).Error("Unable to list event types: %v", zap.Error(err))
		return nil, err
	}
	eventTypes := make([]v1beta1.EventType, 0)
	for _, et := range etl {
		if metav1.IsControlledBy(et, src) {
			eventTypes = append(eventTypes, *et)
		}
	}
	return eventTypes, nil
}

func (r *Reconciler) makeEventTypes(ctx context.Context, src *duckv1.Source) ([]v1beta1.EventType, error) {
	// Only create EventTypes for Broker sinks.
	// We add this check here in case the Source was changed from Broker to non-Broker sink.
	// If so, we need to delete the existing ones, thus we return empty expected.
	// TODO remove broker from EventType https://github.com/knative/eventing/issues/2750
	if ref := src.Spec.Sink.GetRef(); ref == nil || ref.Kind != "Broker" {
		return make([]v1beta1.EventType, 0), nil
	}

	// If the Source didn't specify a CloudEventsAttributes, then we skip the creation of EventTypes.
	// TODO might change in the near future https://github.com/knative/eventing/issues/2750.
	if src.Status.CloudEventAttributes == nil {
		return make([]v1beta1.EventType, 0), nil
	}

	entries := make(map[string]eventTypeEntry)
	// Get the description and schema from the CRD, in case they are there.
	// The CRD annotation has the types as well. But given that different Sources have their own configurations, I'm just
	// grabbing the description and schema from the CRD, using the type as "primary key".
	// By having their own configs I mean, for example, in the GithubSource
	// you can specify the subset of event types you are interested in, or in the PingSource you just have
	// one type, and so on.
	crd, err := r.crdLister.Get(r.crdName)
	if err != nil {
		// Only log, can create the EventType(s) without this info.
		logging.FromContext(ctx).Error("Error getting CRD for Source", zap.Any("src", src))
	} else {
		var ets []eventTypeEntry
		if v, ok := crd.Annotations[eventing.EventTypesAnnotationKey]; ok {
			if err := json.Unmarshal([]byte(v), &ets); err != nil {
				// Same here, only log, can create the EventType(s) without this info.
				logging.FromContext(ctx).Error("Error unmarshalling EventTypes", zap.String("annotation", eventing.EventTypesAnnotationKey), zap.Error(err))
			}
		}
		for _, et := range ets {
			entries[et.Type] = et
		}
	}

	eventTypes := make([]v1beta1.EventType, 0)
	for _, attrib := range src.Status.CloudEventAttributes {
		if attrib.Type == "" {
			// Cannot have empty spec.type
			continue
		}
		var schema, description string
		if v, ok := entries[attrib.Type]; ok {
			schema = v.Schema
			description = v.Description
		}
		sourceURL, err := apis.ParseURL(attrib.Source)
		if err != nil {
			logging.FromContext(ctx).Warn("Failed to parse source as a URL", zap.String("source", attrib.Source), zap.Error(err))
		}
		schemaURL, err := apis.ParseURL(schema)
		if err != nil {
			logging.FromContext(ctx).Warn("Failed to parse schema as a URL", zap.String("schema", schema), zap.Error(err))
		}
		eventType := resources.MakeEventType(&resources.EventTypeArgs{
			Source:      src,
			CeType:      attrib.Type,
			CeSource:    sourceURL,
			CeSchema:    schemaURL,
			Description: description,
		})
		eventTypes = append(eventTypes, *eventType)
	}
	return eventTypes, nil
}

func (r *Reconciler) computeDiff(current []v1beta1.EventType, expected []v1beta1.EventType) ([]v1beta1.EventType, []v1beta1.EventType) {
	toCreate := make([]v1beta1.EventType, 0)
	toDelete := make([]v1beta1.EventType, 0)
	currentMap := asMap(current, keyFromEventType)
	expectedMap := asMap(expected, keyFromEventType)

	// Iterate over the slices instead of the maps for predictable UT expectations.
	for _, e := range expected {
		if c, ok := currentMap[keyFromEventType(&e)]; !ok {
			toCreate = append(toCreate, e)
		} else {
			if !equality.Semantic.DeepEqual(e.Spec, c.Spec) {
				toDelete = append(toDelete, c)
				toCreate = append(toCreate, e)
			}
		}
	}
	// Need to check whether the current EventTypes are not in the expected map. If so, we have to delete them.
	// This could happen if the Source CO changes its broker.
	// TODO remove once we remove Broker https://github.com/knative/eventing/issues/2750
	for _, c := range current {
		if _, ok := expectedMap[keyFromEventType(&c)]; !ok {
			toDelete = append(toDelete, c)
		}
	}
	return toCreate, toDelete
}

func asMap(eventTypes []v1beta1.EventType, keyFunc func(*v1beta1.EventType) string) map[string]v1beta1.EventType {
	eventTypesAsMap := make(map[string]v1beta1.EventType)
	for _, eventType := range eventTypes {
		key := keyFunc(&eventType)
		eventTypesAsMap[key] = eventType
	}
	return eventTypesAsMap
}

// TODO we should probably use the hash of this instead. Will be revisited together with https://github.com/knative/eventing/issues/2750.
func keyFromEventType(eventType *v1beta1.EventType) string {
	return fmt.Sprintf("%s_%s_%s_%s", eventType.Spec.Type, eventType.Spec.Source, eventType.Spec.Schema, eventType.Spec.Broker)
}
