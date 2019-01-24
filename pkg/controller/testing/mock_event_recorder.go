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
)

// MockEventRecorder is a recorder.EventRecorder that saves emitted v1 Events.
type MockEventRecorder struct {
	events []corev1.Event
}

func NewEventRecorder() *MockEventRecorder {
	return &MockEventRecorder{}
}

func (m *MockEventRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	appendEvent(eventtype, reason, m)
}

func (m *MockEventRecorder) Event(object runtime.Object, eventtype, reason, message string) {
	appendEvent(eventtype, reason, m)
}

func (m *MockEventRecorder) PastEventf(object runtime.Object, timestamp metav1.Time, eventtype, reason, messageFmt string, args ...interface{}) {
	appendEvent(eventtype, reason, m)
}

func (m *MockEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
	appendEvent(eventtype, reason, m)
}

// Helper function to append an event type and reason to the MockEventRecorder
// Only interested in type and reason of Events for now. If we are planning on verifying other fields in
// the test cases, we need to include them here.
func appendEvent(eventtype, reason string, m *MockEventRecorder) {
	event := corev1.Event{
		Reason: reason,
		Type:   eventtype,
	}
	m.events = append(m.events, event)
}
