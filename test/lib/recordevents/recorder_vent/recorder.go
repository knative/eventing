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

package recorder_vent

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	"knative.dev/eventing/test/lib/recordevents"
)

type recorder struct {
	out record.EventRecorder
	on  runtime.Object
}

func (r *recorder) Vent(observed recordevents.EventInfo) error {
	b, err := json.Marshal(observed)
	if err != nil {
		return err
	}

	r.out.Eventf(r.on, corev1.EventTypeNormal, recordevents.CloudEventObservedReason, "%s", string(b))

	return nil
}
