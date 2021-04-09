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

package milestone

import (
	"fmt"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

const (
	EnvironmentType      = "dev.knative.rekt.environment.v1"
	NamespaceCreatedType = "dev.knative.rekt.namespace.created.v1"
	NamespaceDeletedType = "dev.knative.rekt.namespace.deleted.v1"
	TestStartedType      = "dev.knative.rekt.test.started.v1"
	TestFinishedType     = "dev.knative.rekt.test.finished.v1"
	StepStartedType      = "dev.knative.rekt.step.started.v1"
	StepFinishedType     = "dev.knative.rekt.step.finished.v1"
	TestSetStartedType   = "dev.knative.rekt.testset.started.v1"
	TestSetFinishedType  = "dev.knative.rekt.testset.finished.v1"
	FinishedType         = "dev.knative.rekt.finished.v1"
	ExceptionType        = "dev.knative.rekt.exception.v1"
)

func NewFactory(id, namespace string) *Factory {
	return &Factory{
		Source:  "knative.dev/reconciler-test/" + id, // TODO: revisit.
		Subject: fmt.Sprintf("/api/v1/namespaces/%s", namespace),
	}
}

type Factory struct {
	Subject string
	Source  string
}

func (ef *Factory) Environment(env map[string]string) cloudevents.Event {
	event := ef.baseEvent(EnvironmentType)

	_ = event.SetData(cloudevents.ApplicationJSON, env)

	return event
}

func (ef *Factory) baseEvent(eventType string) cloudevents.Event {
	event := cloudevents.NewEvent()

	event.SetSubject(ef.Subject)
	event.SetSource(ef.Source)
	event.SetType(eventType)

	return event
}

func (ef *Factory) NamespaceCreated(namespace string) cloudevents.Event {
	event := ef.baseEvent(NamespaceCreatedType)

	_ = event.SetData(cloudevents.ApplicationJSON, map[string]string{
		"namespace": namespace,
	})

	return event
}

func (ef *Factory) NamespaceDeleted(namespace string) cloudevents.Event {
	event := ef.baseEvent(NamespaceDeletedType)

	_ = event.SetData(cloudevents.ApplicationJSON, map[string]string{
		"namespace": namespace,
	})

	return event
}

func (ef *Factory) TestStarted(feature, testName string) cloudevents.Event {
	event := ef.baseEvent(TestStartedType)

	lparts := strings.Split(testName, "/")
	if len(lparts) > 0 {
		event.SetExtension("testparent", lparts[0])
	}

	event.SetExtension("feature", feature)
	event.SetExtension("testname", testName)

	// TODO: we can log a whole lot of stuff here but we need a more formal structure to track
	// where we are in the test to be able to assemble it.

	_ = event.SetData(cloudevents.ApplicationJSON, map[string]interface{}{
		"feature":  feature,
		"testName": testName,
	})

	return event
}

func (ef *Factory) TestFinished(feature, testName string, skipped, failed bool) cloudevents.Event {
	event := ef.baseEvent(TestFinishedType)

	lparts := strings.Split(testName, "/")
	if len(lparts) > 0 {
		event.SetExtension("testparent", lparts[0])
	}

	event.SetExtension("feature", feature)
	event.SetExtension("testname", testName)

	_ = event.SetData(cloudevents.ApplicationJSON, map[string]interface{}{
		"feature":  feature,
		"testName": testName,
		"skipped":  skipped,
		"failed":   failed,
		"passed":   !failed && !skipped,
	})

	return event
}

func (ef *Factory) StepStarted(feature, stepName, timing, level, testName string) cloudevents.Event {
	event := ef.baseEvent(StepStartedType)

	lparts := strings.Split(testName, "/")
	if len(lparts) > 0 {
		event.SetExtension("testparent", lparts[0])
	}

	event.SetExtension("feature", feature)
	event.SetExtension("stepname", stepName)
	event.SetExtension("steptiming", timing)
	event.SetExtension("steplevel", level)
	event.SetExtension("testname", testName)

	// TODO: we can log a whole lot of stuff here but we need a more formal structure to track
	// where we are in the test to be able to assemble it.

	_ = event.SetData(cloudevents.ApplicationJSON, map[string]interface{}{
		"feature":    feature,
		"stepName":   stepName,
		"stepTiming": timing,
		"stepLevel":  level,
		"testName":   testName,
	})

	return event
}

func (ef *Factory) StepFinished(feature, stepName, timing, level, testName string, skipped, failed bool) cloudevents.Event {
	event := ef.baseEvent(StepFinishedType)

	lparts := strings.Split(testName, "/")
	if len(lparts) > 0 {
		event.SetExtension("testparent", lparts[0])
	}

	event.SetExtension("feature", feature)
	event.SetExtension("stepname", stepName)
	event.SetExtension("steptiming", timing)
	event.SetExtension("steplevel", level)
	event.SetExtension("testname", testName)

	_ = event.SetData(cloudevents.ApplicationJSON, map[string]interface{}{
		"feature":    feature,
		"stepName":   stepName,
		"stepTiming": timing,
		"stepLevel":  level,
		"testName":   testName,
		"skipped":    skipped,
		"failed":     failed,
		"passed":     !failed && !skipped,
	})

	return event
}

func (ef *Factory) TestSetStarted(featureSet, testName string) cloudevents.Event {
	event := ef.baseEvent(TestSetStartedType)

	event.SetExtension("featureset", featureSet)
	event.SetExtension("testname", testName)

	// TODO: we can log a whole lot of stuff here but we need a more formal structure to track
	// where we are in the test to be able to assemble it.

	_ = event.SetData(cloudevents.ApplicationJSON, map[string]interface{}{
		"featureSet": featureSet,
		"testName":   testName,
	})

	return event
}

func (ef *Factory) TestSetFinished(featureSet, testName string, skipped, failed bool) cloudevents.Event {
	event := ef.baseEvent(TestSetFinishedType)

	event.SetExtension("featureset", featureSet)

	_ = event.SetData(cloudevents.ApplicationJSON, map[string]interface{}{
		"featureSet": featureSet,
		"testName":   testName,
		"skipped":    skipped,
		"failed":     failed,
		"passed":     !failed && !skipped,
	})

	return event
}

func (ef *Factory) Finished() cloudevents.Event {
	event := ef.baseEvent(FinishedType)

	_ = event.SetData(cloudevents.ApplicationJSON, map[string]string{})

	return event
}

func (ef *Factory) Exception(reason, messageFormat string, messageA ...interface{}) cloudevents.Event {
	event := ef.baseEvent(ExceptionType)

	event.SetExtension("reason", reason)

	_ = event.SetData(cloudevents.ApplicationJSON, map[string]string{
		"reason":  reason,
		"message": fmt.Sprintf(messageFormat, messageA...),
	})

	return event
}
