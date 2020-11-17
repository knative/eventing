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

		// try to unmarshal v1beta2.PingSource.Spec from V1B2SpecAnnotationKey
		v1beta2Spec, ok := annotations[V1B2SpecAnnotationKey]
		if ok {
			if err := json.Unmarshal([]byte(v1beta2Spec), &sink.Spec); err != nil {
				ok = false
			}
			// we don't need this annotation in a v1beta2.PingSource object
			delete(annotations, V1B2SpecAnnotationKey)
		}

		// if V1B2SpecAnnotationKey does not exist or we cannot unmarshal it, do a normal conversion
		if !ok {
			sink.Spec = v1beta2.PingSourceSpec{
				SourceSpec: source.Spec.SourceSpec,
				Schedule:   source.Spec.Schedule,
				Timezone:   source.Spec.Timezone,
			}

			if source.Spec.JsonData != "" {
				msg, err := makeMessage(source.Spec.JsonData)
				if err != nil {
					return fmt.Errorf("error converting jsonData to a higher version: %v", err)
				}
				sink.Spec.ContentType = cloudevents.ApplicationJSON
				sink.Spec.Data = string(msg)
			}
		}

		// marshal and store v1beta1.PingSource.Spec into V1B1SpecAnnotationKey
		// this is to help if we need to convert back to v1beta1.PingSource
		v1beta1Spec, err := json.Marshal(source.Spec)
		if err != nil {
			return fmt.Errorf("error marshalling source.Spec: %v, err: %v", source.Spec, err)
		}
		annotations[V1B1SpecAnnotationKey] = string(v1beta1Spec)
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

		// v1beta2 objects originally converted from v1beta1 is expected to have this annotation
		v1beta1Spec, ok := annotations[V1B1SpecAnnotationKey]

		if ok {
			if err := json.Unmarshal([]byte(v1beta1Spec), &sink.Spec); err != nil {
				return fmt.Errorf("error unmarshalling annotation %v=%v into %T: %v", V1B1SpecAnnotationKey, v1beta1Spec, sink.Spec, err)
			}
			// we don't need this annotation in a v1beta1.PingSource object
			delete(annotations, V1B1SpecAnnotationKey)
		}

		// marshal and store v1beta2.PingSource.Spec into V1B2SpecAnnotationKey
		// this is to help if we need to convert back to v1beta2.PingSource
		v1beta2Configuration, err := json.Marshal(source.Spec)
		if err != nil {
			return fmt.Errorf("error marshalling source.Spec: %v, err: %v", source.Spec, err)
		}
		annotations[V1B2SpecAnnotationKey] = string(v1beta2Configuration)
		sink.SetAnnotations(annotations)

		return nil
	default:
		return apis.ConvertFromViaProxy(ctx, source, &v1beta2.PingSource{}, sink)
	}
}
