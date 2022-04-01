/*
Copyright 2022 The Knative Authors

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

package event

import (
	"io"
	"net/http"
	"net/http/httptest"

	"testing"

	"github.com/openzipkin/zipkin-go/model"
	"github.com/stretchr/testify/assert"
)

func TestTraceParsing(t *testing.T) {
	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			// Using a reduced variant of the response at https://zipkin.io/zipkin-api/#/default/get_traces
			io.WriteString(w,
				`[
  [
    {
      "id": "352bff9a74ca9ad2",
      "traceId": "5af7183fb1d4cf5f",
      "name": "get /api",
      "timestamp": 1556604172355737,
      "duration": 1431,
      "kind": "SERVER",
      "tags": {
        "http.method": "GET",
        "http.path": "/api"
      }
    },
	{
      "id": "352bff9a74ca9ad3",
      "traceId": "5af7183fb1d4cf60",
      "parentId": "352bff9a74ca9ad2",
      "name": "get /api",
      "timestamp": 1556604172355737,
      "duration": 1431,
      "kind": "SERVER",
      "tags": {
        "http.method": "GET",
        "http.path": "/api"
      }
    }
  ]
]`)
		}))
	defer ts.Close()
	trace, err := SendTraceQuery(ts.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	assert.Len(t, trace, 1)
	assert.Len(t, trace[0], 2)
	assert.Equal(t, model.Kind("SERVER"), trace[0][0].Kind)
}
