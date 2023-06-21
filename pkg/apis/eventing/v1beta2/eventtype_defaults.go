/*
Copyright 2021 The Knative Authors

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

package v1beta2

import (
	"context"
)

func (et *EventType) SetDefaults(ctx context.Context) {
	et.Spec.SetDefaults(ctx)
	setReferenceNs(et)
}

func (ets *EventTypeSpec) SetDefaults(ctx context.Context) {
}

func setReferenceNs(et *EventType) {
	if et.Spec.Reference != nil && et.Spec.Reference.Namespace == "" {
		et.Spec.Reference.Namespace = et.GetNamespace()
	}
}
