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

	"knative.dev/eventing/pkg/apis/messaging"
)

func (imc *InMemoryChannel) SetDefaults(ctx context.Context) {
	// Set the duck subscription to indicate that we support
	// v1beta1 version of the Subscribable.
	if imc.Annotations == nil {
		imc.Annotations = make(map[string]string)
	}
	imc.Annotations[messaging.SubscribableDuckVersionAnnotation] = "v1beta1"

	imc.Spec.SetDefaults(ctx)
}

func (imcs *InMemoryChannelSpec) SetDefaults(ctx context.Context) {
	// TODO: Nothing to default here...
}
