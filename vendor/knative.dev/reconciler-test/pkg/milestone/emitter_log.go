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
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"

	"knative.dev/reconciler-test/pkg/feature"
)

type logEmitter struct {
	ctx       context.Context
	namespace string
	t         feature.T
}

// NewLogEmitter creates an Emitter that logs milestone events.
func NewLogEmitter(ctx context.Context, namespace string, t feature.T) Emitter {
	return &logEmitter{ctx: ctx, namespace: namespace, t: t}
}

func (l logEmitter) Environment(env map[string]string) {
	bytes, err := json.MarshalIndent(env, " ", " ")
	if err != nil {
		l.log(nil, err)
		return
	}
	l.log(nil, "Environment", "Namespace", l.namespace, "\n", string(bytes))
}

func (l logEmitter) NamespaceCreated(namespace string) {
	l.log(nil, "Namespace created", namespace)
}

func (l logEmitter) NamespaceDeleted(namespace string) {
	l.log(nil, "Namespace deleted", namespace)
}

func (l logEmitter) TestStarted(feature string, t feature.T) {
	l.log(t, feature, "Test started")
}

func (l logEmitter) TestFinished(feature string, t feature.T) {
	l.log(t, feature, "Test Finished")
}

func (l logEmitter) StepsPlanned(feature string, steps map[feature.Timing][]feature.Step, t feature.T) {
	bytes, err := json.MarshalIndent(steps, " ", " ")
	if err != nil {
		l.log(t, err)
		return
	}
	l.log(t, feature, "Steps Planned", "\n", string(bytes))
}

func (l logEmitter) StepStarted(feature string, step *feature.Step, t feature.T) {
	bytes, err := json.MarshalIndent(step, " ", " ")
	if err != nil {
		l.log(t, err)
		return
	}
	l.log(t, feature, "Step Started", "\n", string(bytes))
}

func (l logEmitter) StepFinished(feature string, step *feature.Step, t feature.T) {
	bytes, err := json.MarshalIndent(step, " ", " ")
	if err != nil {
		l.log(t, err)
		return
	}
	l.log(t, feature, "Step Finished", "\n", string(bytes))
}

func (l logEmitter) TestSetStarted(featureSet string, t feature.T) {
	l.log(t, featureSet, "FeatureSet Started")
}

func (l logEmitter) TestSetFinished(featureSet string, t feature.T) {
	l.log(t, featureSet, "FeatureSet Finished")
}

func (l logEmitter) Finished() {
	l.log(nil, "Finished")
	l.dumpEvents()
}

func (l logEmitter) Exception(reason, messageFormat string, messageA ...interface{}) {
	l.log(nil, "Exception", reason, fmt.Sprintf(messageFormat, messageA...))
}

func (l logEmitter) dumpEvents() {
	if l.t == nil {
		return
	}
	events, err := kubeclient.Get(l.ctx).CoreV1().Events(l.namespace).List(l.ctx, metav1.ListOptions{})
	if err != nil {
		l.log(nil, "failed to list events", err)
		return
	}
	if len(events.Items) == 0 {
		l.log(nil, "No events found")
		return
	}
	for _, e := range sortEventsByTime(events.Items) {
		l.log(nil, formatEvent(e))
	}
}

func sortEventsByTime(items []corev1.Event) []corev1.Event {
	sort.SliceStable(items, func(i, j int) bool {
		return eventTime(items[i]).Before(eventTime(items[j]))
	})
	return items
}

func eventTime(e corev1.Event) time.Time {
	// Some events might not contain last timestamp, in that case
	// we fall back to the event time.
	if e.LastTimestamp.Time.IsZero() {
		return e.EventTime.Time
	}
	return e.LastTimestamp.Time
}

func formatEvent(e corev1.Event) string {
	return strings.Join([]string{`Event{`,
		`ObjectMeta:` + strings.Replace(e.ObjectMeta.String(), `&`, ``, 1),
		`InvolvedObject:` + strings.Replace(e.InvolvedObject.String(), `&`, ``, 1),
		`Reason:` + e.Reason,
		`Message:` + e.Message,
		`Source:` + strings.Replace(e.Source.String(), `&`, ``, 1),
		`FirstTimestamp:` + e.FirstTimestamp.String(),
		`LastTimestamp:` + e.LastTimestamp.String(),
		`Count:` + fmt.Sprintf("%d", e.Count),
		`Type:` + e.Type,
		`EventTime:` + e.EventTime.String(),
		`Series:` + e.Series.String(),
		`Action:` + e.Action,
		`Related:` + e.Related.String(),
		`ReportingController:` + e.ReportingController,
		`ReportingInstance:` + e.ReportingInstance,
		`}`,
	}, "\n")
}

func (l logEmitter) log(t feature.T, args ...interface{}) {
	ctxArgs := []interface{}{
		"namespace", l.namespace,
		"timestamp", time.Now().Format(time.RFC3339),
		"\n",
	}
	if t == nil {
		t = l.t
	}
	args = append(ctxArgs, args...)
	if l.t != nil {
		l.t.Log(args...)
	}
}
