/*
Copyright 2025 The Knative Authors

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
	"testing"

	"github.com/stretchr/testify/assert"
	"knative.dev/pkg/webhook/json"
)

func TestJSONDecode(t *testing.T) {

	et := &EventTransform{}

	err := json.Decode([]byte(`
{
  "apiVersion": "eventing.knative.dev/v1alpha1",
  "kind": "EventTransform",
  "metadata": {
    "name": "identity"
  },
  "spec": {
    "jsonata": {
      "expression": "{\n  \"specversion\": \"1.0\",\n  \"id\": id,\n  \"type\": \"transformation.jsonata\",\n  \"source\": \"transformation.json.identity\",\n  \"data\": $\n}\n"
    }
  }
}
`), et, true)

	assert.Nil(t, err)
}
