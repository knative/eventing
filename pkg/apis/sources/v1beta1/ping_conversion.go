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

package v1beta1

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"knative.dev/eventing/pkg/apis/sources/v1beta2"
	"knative.dev/pkg/apis"
)

const (
	// V1B1SpecAnnotationKey is used to indicate that a v1beta2 object is converted from v1beta1
	// also it can be used to downgrade such object to v1beta1
	V1B1SpecAnnotationKey = "pingsources.sources.knative.dev/v1beta1-spec"

	// V1B2SpecAnnotationKey is used to indicate that a v1beta1 object is converted from v1beta2
	// also it can be used to convert the v1beta1 object back to v1beta2, considering that v1beta2 introduces more features.
	V1B2SpecAnnotationKey = "pingsources.sources.knative.dev/v1beta2-spec"
)

type message struct {
	Body string `json:"body"`
}

func makeMessage(body string) ([]byte, error) {
	// try to marshal the body into an interface.
	var objmap map[string]*json.RawMessage
	if err := json.Unmarshal([]byte(body), &objmap); err != nil {
		// Default to a wrapped message.
		return json.Marshal(message{Body: body})
	}
	return json.Marshal(objmap)
}

// ConvertTo implements apis.Convertible
// Converts source from v1beta1.PingSource into a higher version.
func (source *PingSource) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1beta2.PingSource:
		sink.ObjectMeta = source.ObjectMeta
		sink.Status = v1beta2.PingSourceStatus{
			SourceStatus: source.Status.SourceStatus,
		}

		// deep copy annotations to avoid mutation on source.ObjectMeta.Annotations
		annotations := make(map[string]string)
		for key, value := range source.GetAnnotations() {
			annotations[key] = value
		}

		if isCreatedViaV1Beta2API(source) {
			// try to unmarshal v1beta2.PingSource.Spec from V1B2SpecAnnotationKey
			// key existence and json marshal error already checked in isCreatedViaV1Beta2API
			v1beta2Spec := annotations[V1B2SpecAnnotationKey]
			_ = json.Unmarshal([]byte(v1beta2Spec), &sink.Spec)
		} else {
			var err error
			if sink.Spec, err = toV1Beta2Spec(&source.Spec); err != nil {
				return err
			}
			// marshal and store v1beta1.PingSource.Spec into V1B1SpecAnnotationKey
			// this is to help if we need to convert back to v1beta1.PingSource
			v1beta1Spec, err := json.Marshal(source.Spec)
			if err != nil {
				return fmt.Errorf("error marshalling source.Spec: %v, err: %v", source.Spec, err)
			}
			annotations[V1B1SpecAnnotationKey] = string(v1beta1Spec)
		}

		// we don't need this annotation in a v1beta2.PingSource object
		delete(annotations, V1B2SpecAnnotationKey)
		sink.SetAnnotations(annotations)
		return nil
	default:
		return apis.ConvertToViaProxy(ctx, source, &v1beta2.PingSource{}, sink)
	}
}

// ConvertFrom implements apis.Convertible
// Converts obj from a higher version into v1beta1.PingSource.
func (sink *PingSource) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1beta2.PingSource:
		sink.ObjectMeta = source.ObjectMeta
		sink.Status = PingSourceStatus{
			SourceStatus: source.Status.SourceStatus,
		}

		// deep copy annotations to avoid mutation on source.ObjectMeta.Annotations
		annotations := make(map[string]string)
		for key, value := range source.GetAnnotations() {
			annotations[key] = value
		}

		if isV1Beta1AnnotationConsistentWithV1Beta2Spec(source) {
			// errors already handled in isV1Beta1AnnotationConsistentWithV1Beta2Spec
			v1beta1Spec := annotations[V1B1SpecAnnotationKey]
			_ = json.Unmarshal([]byte(v1beta1Spec), &sink.Spec)
		}

		// marshal and store v1beta2.PingSource.Spec into V1B2SpecAnnotationKey
		// this is to help if we need to convert back to v1beta2.PingSource
		v1beta2Configuration, err := json.Marshal(source.Spec)
		if err != nil {
			return fmt.Errorf("error marshalling source.Spec: %v, err: %v", source.Spec, err)
		}
		annotations[V1B2SpecAnnotationKey] = string(v1beta2Configuration)
		// we don't need this annotation in a v1beta1.PingSource object
		delete(annotations, V1B1SpecAnnotationKey)
		sink.SetAnnotations(annotations)

		return nil
	default:
		return apis.ConvertFromViaProxy(ctx, source, &v1beta2.PingSource{}, sink)
	}
}

func toV1Beta2Spec(srcSpec *PingSourceSpec) (v1beta2.PingSourceSpec, error) {
	targetSpec := v1beta2.PingSourceSpec{
		SourceSpec: srcSpec.SourceSpec,
		Schedule:   srcSpec.Schedule,
		Timezone:   srcSpec.Timezone,
	}

	if srcSpec.JsonData != "" {
		msg, err := makeMessage(srcSpec.JsonData)
		if err != nil {
			return targetSpec, fmt.Errorf("error converting jsonData to a higher version: %v", err)
		}
		targetSpec.ContentType = cloudevents.ApplicationJSON
		targetSpec.Data = string(msg)
	}

	return targetSpec, nil
}

// checks if a v1beta1.PingSource is originally created in v1beta2, it must meet both of the following criteria:
//
// 1. V1B2SpecAnnotationKey annotation must exist and can be unmarshalled to v1beta2.PingSourceSpec, it indicates that it's converted from v1beta2 -> v1beta1.
// 2. Spec.Sink must be {Ref: nil, URI: nil}, as we don't set these values during conversion from v1beta2 -> v1beta1, see PingSource.ConvertFrom;
func isCreatedViaV1Beta2API(source *PingSource) bool {
	v1beta2Annotation, ok := source.GetAnnotations()[V1B2SpecAnnotationKey]
	if !ok {
		return false
	}

	v1beta2Spec := &v1beta2.PingSourceSpec{}
	if err := json.Unmarshal([]byte(v1beta2Annotation), v1beta2Spec); err != nil {
		return false
	}

	return source.Spec.Sink.Ref == nil && source.Spec.Sink.URI == nil
}

// for a v1beta2.PingSource, checks if its V1B1SpecAnnotationKey is consistent with its spec.
// returns false if one of the following satisfies:
//
// 1. V1B1SpecAnnotationKey does not exist.
// 2. V1B1SpecAnnotationKey exists, but we cannot unmarshal it to v1beta1.PingSourceSpec.
// 3. V1B1SpecAnnotationKey exists, but if we unmarshal it to v1beta1.PingSourceSpec and convert it to v1beta2,
// the converted v1beta2.PingSourceSpec is not the same as source.Spec.
func isV1Beta1AnnotationConsistentWithV1Beta2Spec(source *v1beta2.PingSource) bool {
	v1beta1Annotation, ok := source.GetAnnotations()[V1B1SpecAnnotationKey]
	if !ok {
		return false
	}

	v1beta1Spec := &PingSourceSpec{}
	if err := json.Unmarshal([]byte(v1beta1Annotation), v1beta1Spec); err != nil {
		return false
	}

	v1beta2Spec, err := toV1Beta2Spec(v1beta1Spec)
	if err != nil {
		return false
	}

	return reflect.DeepEqual(v1beta2Spec, source.Spec)
}
