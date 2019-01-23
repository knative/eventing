/*
Copyright 2018 The Knative Authors

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

package testing

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

var _ record.EventRecorder = (*MockEventRecorder)(nil)

// mockEventRecorder is a recorder.EventRecorder that allows to save v1 Events emitted.
type MockEventRecorder struct {
	events []corev1.Event
}

func NewEventRecorder() *MockEventRecorder {
	return &MockEventRecorder{}
}

func (m *MockEventRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	// Create an event with only the information that we need to verify in the test.
	// Should include more information if we want to check other fields
	event := corev1.Event{
		Reason: reason,
		Type:   eventtype,
	}
	m.events = append(m.events, event)
}

func (m *MockEventRecorder) Event(object runtime.Object, eventtype, reason, message string) {
	panic("not implemented")
}

func (m *MockEventRecorder) PastEventf(object runtime.Object, timestamp metav1.Time, eventtype, reason, messageFmt string, args ...interface{}) {
	panic("not implemented")
}

func (m *MockEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
	panic("not implemented")
}
