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

	"knative.dev/reconciler-test/pkg/feature"
)

type tracingEmitter struct {
}

// NewTracingGatherer implements Emitter and gathers traces for events from the tracing entpoint.
// TODO(Cali0707): get this to work with the new OTel changes
func NewTracingGatherer(ctx context.Context, namespace string, zipkinNamespace string, t feature.T) (Emitter, error) {
	return &tracingEmitter{}, nil
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
}

func (e tracingEmitter) Exception(reason, messageFormat string, messageA ...interface{}) {
}
