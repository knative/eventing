/*
Copyright 2018 Google, Inc. All rights reserved.

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

package event_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/elafros/eventing/pkg/event"
)

var (
	context = &event.Context{
		CloudEventsVersion: "0.0.1-preview-1",
		EventId:            "eventid-123",
		EventType:          "google.firestore.document.create",
		EventTime:          time.Now().UTC(),
		Source:             "//firestore.googleapis.com/projects/demo/databases/default/documents/users/inlined",
	}
	data = map[string]interface{}{
		"username":      "inlined",
		"project":       "eventing",
		"emailVerified": false,
	}
)

func TestEncoding(t *testing.T) {
	req, err := event.NewRequest("http://localhost/:sendEvent", data, *context)
	if err != nil {
		t.Fatalf("Failed to encode event %s", err)
	}

	var foundData map[string]interface{}
	foundContext, err := event.FromRequest(&foundData, req)
	if err != nil {
		t.Fatalf("Failed to decode event %s", err)
	}

	if !reflect.DeepEqual(context, foundContext) {
		t.Fatalf("Context was transcoded lossily: expected=%+v got=%+v", context, foundContext)
	}
	if !reflect.DeepEqual(data, foundData) {
		t.Fatalf("Data was transcoded lossily: expected=%+v got=%+v", data, foundData)
	}
}
