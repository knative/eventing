/*
Copyright 2020 The Knative Authors.

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

package v1alpha1

import (
	"context"
	"encoding/json"
	"fmt"

	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/pkg/apis"
)

// ConvertTo implements apis.Convertible.
// Converts source (from v1alpha1.EventType) into v1beta1.EventType
func (source *EventType) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1beta1.EventType:
		b, err := json.Marshal(source)
		if err != nil {
			return err
		}
		return json.Unmarshal(b, sink)
	default:
		return fmt.Errorf("Unknown conversion, got: %T", sink)

	}
}

// ConvertFrom implements apis.Convertible.
// Converts obj from v1beta1.EventType into v1alpha1.EventType
func (sink *EventType) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1beta1.EventType:
		b, err := json.Marshal(source)
		if err != nil {
			return err
		}
		return json.Unmarshal(b, sink)
	default:
		return fmt.Errorf("Unknown conversion, got: %T", source)
	}
}
