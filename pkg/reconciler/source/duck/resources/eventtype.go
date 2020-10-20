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

package resources

import (
	"crypto/md5" //nolint:gosec // No strong cryptography needed.
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"
)

// EventTypeArgs are the arguments needed to create an EventType for a Source.
type EventTypeArgs struct {
	Source      *duckv1.Source
	CeType      string
	CeSource    *apis.URL
	CeSchema    *apis.URL
	Description string
}

func MakeEventType(args *EventTypeArgs) *v1beta1.EventType {
	// Name it with the hash of the concatenation of the three fields.
	// Cannot generate a fixed name based on type+UUID, because long type names might be cut, and we end up trying to create
	// event types with the same name.
	// TODO revisit whether we want multiple event types, or just one with multiple owner refs. That will depend on the fields
	//  it will contain. For example, if we remove Broker and Source, then the latter makes more sense.
	//  See https://github.com/knative/eventing/issues/2750
	fixedName := fmt.Sprintf("%x", md5.Sum([]byte(args.CeType+args.CeSource.String()+args.CeSchema.String()+string(args.Source.GetUID())))) //nolint:gosec // No strong cryptography needed.
	return &v1beta1.EventType{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fixedName,
			Labels:    Labels(args.Source.Name),
			Namespace: args.Source.Namespace,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         args.Source.APIVersion,
				Kind:               args.Source.Kind,
				Name:               args.Source.Name,
				UID:                args.Source.UID,
				BlockOwnerDeletion: ptr.Bool(true),
				Controller:         ptr.Bool(true),
			}},
		},
		Spec: v1beta1.EventTypeSpec{
			Type:   args.CeType,
			Source: args.CeSource,
			// TODO remove broker https://github.com/knative/eventing/issues/2750
			Broker:      args.Source.Spec.Sink.GetRef().Name,
			Description: args.Description,
			Schema:      args.CeSchema,
		},
	}
}
