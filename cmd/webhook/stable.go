// +build !js_trigger_filter

package main

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/webhook/resourcesemantics"
	"knative.dev/pkg/webhook/resourcesemantics/conversion"
)

func AdditionalCRD() map[schema.GroupVersionKind]resourcesemantics.GenericCRD {
	return map[schema.GroupVersionKind]resourcesemantics.GenericCRD{}
}

func AdditionalConversions() map[schema.GroupKind]conversion.GroupKindConversion {
	return map[schema.GroupKind]conversion.GroupKindConversion{}
}
