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

package milestone

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/openzipkin/zipkin-go/model"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/test/helpers"
	"knative.dev/pkg/test/zipkin"

	"knative.dev/reconciler-test/pkg/feature"
)

const defaultTracesEndpoint = "http://localhost:9411/api/v2/traces"

type tracingEmitter struct {
	ctx       context.Context
	namespace string
	t         feature.T
}

// NewTracingGatherer implements Emitter and gathers traces for events from the tracing entpoint.
func NewTracingGatherer(ctx context.Context, namespace string, zipkinNamespace string, t feature.T) (Emitter, error) {
	err := zipkin.SetupZipkinTracingFromConfigTracing(ctx,
		kubeclient.Get(ctx),
		logging.FromContext(ctx).Infof,
		zipkinNamespace)
	return &tracingEmitter{ctx: ctx, namespace: namespace, t: t}, err
}

func (e tracingEmitter) Environment(env map[string]string) {
}

func (e tracingEmitter) NamespaceCreated(namespace string) {
}

func (e tracingEmitter) NamespaceDeleted(namespace string) {
}

func (e tracingEmitter) TestStarted(feature string, t feature.T) {
}

func (e tracingEmitter) TestFinished(feature string, t feature.T) {
}

func (e tracingEmitter) StepsPlanned(feature string, steps map[feature.Timing][]feature.Step, t feature.T) {
}

func (e tracingEmitter) StepStarted(feature string, step *feature.Step, t feature.T) {
}

func (e tracingEmitter) StepFinished(feature string, step *feature.Step, t feature.T) {
}

func (e tracingEmitter) TestSetStarted(featureSet string, t feature.T) {
}

func (e tracingEmitter) TestSetFinished(featureSet string, t feature.T) {
}

func (e tracingEmitter) Finished(result Result) {
	if !result.Failed() {
		// Don't export traces on successful runs.
		return
	}
	log := logging.FromContext(e.ctx)
	trace, err := e.getTracesForNamespace(e.namespace)
	if err != nil {
		log.Warnf("Unable to fetch traces for namespace %s: %v", e.namespace, err)
	}
	if err := e.exportTrace(trace, fmt.Sprintf("%s.json", e.namespace)); err != nil {
		log.Warnf("Failed to export traces for namespace %s: %v", e.namespace, err)
	}
}

func (e tracingEmitter) Exception(reason, messageFormat string, messageA ...interface{}) {
}

func (e tracingEmitter) getTracesForNamespace(ns string) ([]byte, error) {
	logging.FromContext(e.ctx).Infof("Fetching traces for namespace %s", ns)
	query := fmt.Sprintf("namespace=%s", ns)
	trace, err := e.findTrace(query)
	if err != nil {
		return nil, err
	}
	return trace, nil
}

func (e tracingEmitter) exportTrace(trace []byte, fileName string) error {
	tracesDir := filepath.Join(getLocalArtifactsDir(), "traces")
	if err := helpers.CreateDir(tracesDir); err != nil {
		return fmt.Errorf("error creating directory %q: %w", tracesDir, err)
	}
	fp := filepath.Join(tracesDir, fileName)
	logging.FromContext(e.ctx).Infof("Exporting trace into %s", fp)
	f, err := os.Create(fp)
	if err != nil {
		return fmt.Errorf("error creating file %q: %w", fp, err)
	}
	defer f.Close()
	_, err = f.Write(trace)
	if err != nil {
		return fmt.Errorf("error writing trace into file %q: %w", fp, err)
	}
	return nil
}

// getLocalArtifactsDir gets the artifacts directory where prow looks for artifacts.
// By default, it will look at the env var ARTIFACTS.
func getLocalArtifactsDir() string {
	dir := os.Getenv("ARTIFACTS")
	if dir == "" {
		dir = "artifacts"
		log.Printf("Env variable ARTIFACTS not set. Using %s instead.", dir)
	}
	return dir
}

// findTrace fetches tracing endpoint and retrieves Zipkin traces matching the annotation query.
func (e tracingEmitter) findTrace(annotationQuery string) ([]byte, error) {
	trace := []byte("{}")
	spanModelsAll, err := e.sendTraceQuery(defaultTracesEndpoint, annotationQuery)
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

// sendTraceQuery sends the query to the tracing endpoint and returns all spans matching the query.
func (e tracingEmitter) sendTraceQuery(endpoint string, annotationQuery string) ([][]model.SpanModel, error) {
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

	body, err := ioutil.ReadAll(resp.Body)
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
