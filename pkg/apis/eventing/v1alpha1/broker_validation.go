/*
Copyright 2019 The Knative Authors

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
	"github.com/knative/pkg/apis"
)

func (b *Broker) Validate(ctx context.Context) *apis.FieldError {
	return b.Spec.Validate(ctx).ViaField("spec")
}

func (bs *BrokerSpec) Validate(ctx context.Context) *apis.FieldError {
	// TODO validate that the channelTemplate only specifies the provisioner and arguments.
	return nil
}

func (b *Broker) CheckImmutableFields(ctx context.Context, og apis.Immutable) *apis.FieldError {
	// Currently there are no immutable fields. We could make spec.channelTemplate immutable, as
	// changing it will normally not have the desired effect of changing the Channel inside the
	// Broker. It would have an effect if the existing Channel was then deleted, the newly created
	// Channel would use the new spec.channelTemplate.
	return nil
}
