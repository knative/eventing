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
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/openzipkin/zipkin-go/model"
)

const ZipkinTracesEndpoint = "http://localhost:9411/api/v2/traces"

// FindTrace fetches tracing endpoint and retrieves Zipkin traces matching the annotation query.
func FindTrace(annotationQuery string) ([]byte, error) {
	trace := []byte("<unavailable>")
	spanModelsAll, err := SendTraceQuery(ZipkinTracesEndpoint, annotationQuery)
	if err != nil {
		return trace, fmt.Errorf("failed to send trace query %q: %w", annotationQuery, err)
	}
	if len(spanModelsAll) == 0 {
		return trace, fmt.Errorf("no traces found for query %q", annotationQuery)
	}
	var models []model.SpanModel
	for _, m := range spanModelsAll {
		models = append(models, m...)
	}
	b, err := json.MarshalIndent(models, "", "  ")
	if err != nil {
		return trace, fmt.Errorf("failed to marshall span models: %w", err)
	}
	return b, nil
}

// SendTraceQuery sends the query to the tracing endpoint and returns all spans matching the query.
func SendTraceQuery(endpoint string, annotationQuery string) ([][]model.SpanModel, error) {
	var empty [][]model.SpanModel
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return empty, err
	}
	q := req.URL.Query()
	q.Add("annotationQuery", annotationQuery)
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return empty, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return empty, err
	}

	var models [][]model.SpanModel
	err = json.Unmarshal(body, &models)
	if err != nil {
		return empty, fmt.Errorf("got an error in unmarshalling JSON %q: %w", body, err)
	}

	return models, nil
}
