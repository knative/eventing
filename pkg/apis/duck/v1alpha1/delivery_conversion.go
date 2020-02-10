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

	"knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/apis"
)

// ConvertUp implements apis.Convertible.
// Converts source (from v1alpha1.DeliverySpec) into v1beta1.DeliverySpec
func (source *DeliverySpec) ConvertUp(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1beta1.DeliverySpec:
		b, err := json.Marshal(source)
		if err != nil {
			return err
		}
		return json.Unmarshal(b, sink)
	default:
		return fmt.Errorf("Unknown conversion, got: %T", sink)

	}
}

// ConvertDown implements apis.Convertible.
// Converts obj from v1beta1.DeliverySpec into v1alpha1.DeliverySpec
func (sink *DeliverySpec) ConvertDown(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1beta1.DeliverySpec:
		b, err := json.Marshal(source)
		if err != nil {
			return err
		}
		return json.Unmarshal(b, sink)
	default:
		return fmt.Errorf("Unknown conversion, got: %T", source)
	}
}

// ConvertUp implements apis.Convertible.
// Converts source (from v1alpha1.DeliveryStatus) into v1beta1.DeliveryStatus
func (source *DeliveryStatus) ConvertUp(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1beta1.DeliveryStatus:
		b, err := json.Marshal(source)
		if err != nil {
			return err
		}
		return json.Unmarshal(b, sink)
	default:
		return fmt.Errorf("Unknown conversion, got: %T", sink)

	}
}

// ConvertDown implements apis.Convertible.
// Converts obj from v1beta1.DeliveryStatus into v1alpha1.DeliveryStatus
func (sink *DeliveryStatus) ConvertDown(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1beta1.DeliveryStatus:
		b, err := json.Marshal(source)
		if err != nil {
			return err
		}
		return json.Unmarshal(b, sink)
	default:
		return fmt.Errorf("Unknown conversion, got: %T", source)
	}
}
