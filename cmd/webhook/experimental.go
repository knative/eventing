// +build js_trigger_filter

package main

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/webhook/resourcesemantics"
	"knative.dev/pkg/webhook/resourcesemantics/conversion"

	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	eventingv1experimental "knative.dev/eventing/pkg/apis/eventing/v1experimental"
)

func AdditionalCRD() map[schema.GroupVersionKind]resourcesemantics.GenericCRD {
	return map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
		eventingv1experimental.SchemeGroupVersion.WithKind("Trigger"): &eventingv1experimental.Trigger{},
	}
}

func AdditionalConversions() map[schema.GroupKind]conversion.GroupKindConversion {
	var (
		eventingv1beta1_        = eventingv1beta1.SchemeGroupVersion.Version
		eventingv1_             = eventingv1.SchemeGroupVersion.Version
		eventingv1experimental_ = eventingv1experimental.SchemeGroupVersion.Version
	)

	return map[schema.GroupKind]conversion.GroupKindConversion{
		eventingv1.Kind("Trigger"): {
			DefinitionName: eventing.TriggersResource.String(),
			HubVersion:     eventingv1experimental_,
			Zygotes: map[string]conversion.ConvertibleObject{
				eventingv1beta1_:        &eventingv1beta1.Trigger{},
				eventingv1_:             &eventingv1.Trigger{},
				eventingv1experimental_: &eventingv1experimental.Trigger{},
			},
		},
	}
}
