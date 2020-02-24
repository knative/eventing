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
	"fmt"

	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/pkg/apis"
)

// ConvertTo implements apis.Convertible.
// Converts source (from v1alpha1.Broker) into v1beta1.Broker
func (source *Broker) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1beta1.Broker:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.Config = source.Spec.Config
		sink.Spec.Delivery = source.Spec.Delivery
		sink.Status.Status = source.Status.Status
		if err := source.Status.Address.ConvertTo(ctx, &sink.Status.Address); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("Unknown conversion, got: %T", sink)

	}
}

// ConvertFrom implements apis.Convertible.
// Converts obj from v1beta1.Broker into v1alpha1.Broker
func (sink *Broker) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1beta1.Broker:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec.Delivery = source.Spec.Delivery
		sink.Status.Status = source.Status.Status
		if err := sink.Status.Address.ConvertFrom(ctx, &source.Status.Address); err != nil {
			return err
		}
		sink.Spec.Config = source.Spec.Config
		return nil
	default:
		return fmt.Errorf("Unknown conversion, got: %T", source)
	}
}
