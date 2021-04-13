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

		sink.Spec = PingSourceSpec{
			SourceSpec: source.Spec.SourceSpec,
			Schedule:   source.Spec.Schedule,
			Timezone:   source.Spec.Timezone,
		}

		if source.Spec.ContentType == cloudevents.ApplicationJSON {
			sink.Spec.JsonData = source.Spec.Data
		}

		return nil
	default:
		return apis.ConvertFromViaProxy(ctx, source, &v1beta2.PingSource{}, sink)
	}
}
