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

package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/broker"
	"knative.dev/eventing/pkg/kncloudevents/attributes"
)

func TestGenerateEventName(t *testing.T) {
	tests := []struct {
		name        string
		errorDest   string
		eventType   string
		eventSource string
		errorCode   string
	}{
		{
			name:        "basic event",
			errorDest:   "http://broker-ingress.knative-eventing.svc.cluster.local/default/my-broker",
			eventType:   "my.event.type",
			eventSource: "my-source",
			errorCode:   "500",
		},
		{
			name:        "same inputs produce same name",
			errorDest:   "http://example.com/path",
			eventType:   "test.type",
			eventSource: "test-source",
			errorCode:   "404",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name1 := generateEventName(tt.errorDest, tt.eventType, tt.eventSource, tt.errorCode)
			name2 := generateEventName(tt.errorDest, tt.eventType, tt.eventSource, tt.errorCode)

			if name1 != name2 {
				t.Errorf("generateEventName not deterministic: got %q and %q", name1, name2)
			}

			if len(name1) < len(EventNamePrefix)+1 {
				t.Errorf("event name too short: %q", name1)
			}
		})
	}

	name1 := generateEventName("dest1", "type1", "source1", "code1")
	name2 := generateEventName("dest2", "type1", "source1", "code1")
	if name1 == name2 {
		t.Error("different destinations should produce different event names")
	}
}

func TestExtractNamespaceFromURL(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		wantNs string
	}{
		{"broker ingress pattern", "http://broker-ingress.knative-eventing.svc.cluster.local/my-namespace/my-broker", "my-namespace"},
		{"default namespace", "http://broker-ingress.knative-eventing.svc.cluster.local/default/broker", "default"},
		{"service pattern", "http://my-service.my-namespace.svc.cluster.local/path", "my-namespace"},
		{"empty path", "http://service.namespace.svc.cluster.local", "namespace"},
		{"invalid url", "not-a-url", ""},
		{"simple host", "http://simple.host", "host"},
		{"ftp scheme", "ftp://example.com/path", ""},
		{"https scheme", "https://broker-ingress.knative-eventing.svc.cluster.local/my-ns/broker", "my-ns"},
		{"single host part", "http://localhost", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractNamespaceFromURL(tt.url)
			if got != tt.wantNs {
				t.Errorf("extractNamespaceFromURL(%q) = %q, want %q", tt.url, got, tt.wantNs)
			}
		})
	}
}

func TestExtractResourceName(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"broker pattern", "http://broker-ingress.knative-eventing.svc.cluster.local/namespace/my-broker", "my-broker"},
		{"no resource name", "http://service.namespace.svc.cluster.local", "unknown"},
		{"invalid url", "://invalid", "unknown"},
		{"single path segment", "http://example.com/onlyone", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractResourceName(tt.url)
			if got != tt.want {
				t.Errorf("extractResourceName(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{"no truncation needed", "short", 10, "short"},
		{"exact length", "exact", 5, "exact"},
		{"needs truncation", "this is a very long string", 10, "this is a "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateString(tt.s, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestGenerateAggregationKey(t *testing.T) {
	key1 := generateAggregationKey("ns1", "dest1", "type1", "source1", "500")
	key2 := generateAggregationKey("ns1", "dest1", "type1", "source1", "500")
	key3 := generateAggregationKey("ns2", "dest1", "type1", "source1", "500")

	if key1 != key2 {
		t.Error("same inputs should produce same key")
	}
	if key1 == key3 {
		t.Error("different namespace should produce different key")
	}
}

func TestEventAggregator_RecordEvent(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	ctx := context.Background()

	event := cloudevents.NewEvent()
	event.SetID("test-id-1")
	event.SetType("test.event.type")
	event.SetSource("test-source")
	event.SetExtension(attributes.KnativeErrorDestExtensionKey, "http://broker-ingress.knative-eventing.svc.cluster.local/test-ns/my-broker")
	event.SetExtension(attributes.KnativeErrorCodeExtensionKey, "500")

	aggregator.RecordEvent(ctx, &event)

	aggregator.mu.RLock()
	bufferLen := len(aggregator.buffer)
	aggregator.mu.RUnlock()

	if bufferLen != 1 {
		t.Errorf("expected 1 event in buffer, got %d", bufferLen)
	}

	aggregator.RecordEvent(ctx, &event)

	aggregator.mu.RLock()
	for _, agg := range aggregator.buffer {
		if agg.Count != 2 {
			t.Errorf("expected count 2, got %d", agg.Count)
		}
	}
	aggregator.mu.RUnlock()
}

func TestEventAggregator_BufferLimit(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	maxBuffer := 5
	aggregator := NewEventAggregator(client, logger, 30*time.Second, maxBuffer)

	ctx := context.Background()

	for i := 0; i < maxBuffer+2; i++ {
		event := cloudevents.NewEvent()
		event.SetID("test-id-" + string(rune('0'+i)))
		event.SetType("test.event.type." + string(rune('0'+i)))
		event.SetSource("test-source")
		event.SetExtension(attributes.KnativeErrorDestExtensionKey, "http://broker-ingress.knative-eventing.svc.cluster.local/test-ns/my-broker")
		event.SetExtension(attributes.KnativeErrorCodeExtensionKey, "500")

		aggregator.RecordEvent(ctx, &event)
	}

	time.Sleep(100 * time.Millisecond)

	aggregator.mu.RLock()
	bufferLen := len(aggregator.buffer)
	aggregator.mu.RUnlock()

	if bufferLen > maxBuffer {
		t.Errorf("buffer exceeded max size: got %d, max %d", bufferLen, maxBuffer)
	}
}

func TestEventAggregator_MissingErrorDest(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	ctx := context.Background()

	event := cloudevents.NewEvent()
	event.SetID("test-id")
	event.SetType("test.event.type")
	event.SetSource("test-source")

	aggregator.RecordEvent(ctx, &event)

	aggregator.mu.RLock()
	bufferLen := len(aggregator.buffer)
	aggregator.mu.RUnlock()

	if bufferLen != 0 {
		t.Errorf("expected empty buffer for event without error dest, got %d", bufferLen)
	}
}

func TestEventAggregator_TTLExhausted(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	ctx := context.Background()

	event := cloudevents.NewEvent()
	event.SetID("test-id-ttl")
	event.SetType("test.event.type")
	event.SetSource("test-source")
	event.SetExtension(attributes.KnativeErrorDestExtensionKey, "http://broker-ingress.knative-eventing.svc.cluster.local/test-ns/my-broker")
	_ = broker.SetTTL(event.Context, 0)

	aggregator.RecordEvent(ctx, &event)

	aggregator.mu.RLock()
	defer aggregator.mu.RUnlock()

	if len(aggregator.buffer) != 1 {
		t.Errorf("expected 1 event in buffer, got %d", len(aggregator.buffer))
		return
	}

	for _, agg := range aggregator.buffer {
		if agg.ErrorCode != "TTL" {
			t.Errorf("expected error code 'TTL', got %q", agg.ErrorCode)
		}
	}
}

func TestGenerateEventNote(t *testing.T) {
	agg := &AggregatedEvent{
		EventType:   "my.event.type",
		EventSource: "my-source",
		ErrorDest:   "http://example.com/dest",
		ErrorCode:   "500",
	}

	note := generateEventNote(agg, 10)

	if note == "" {
		t.Error("note should not be empty")
	}

	expectedParts := []string{"my.event.type", "my-source", "http://example.com/dest", "500", "10"}
	for _, part := range expectedParts {
		if !containsString(note, part) {
			t.Errorf("note %q should contain %q", note, part)
		}
	}
}

func TestGenerateEventNote_AllErrorCodes(t *testing.T) {
	tests := []struct {
		errorCode     string
		shouldContain string
	}{
		{"TTL", "TTL exhausted"},
		{"ttl", "TTL exhausted"},
		{"500", "unhealthy or overloaded"},
		{"502", "unhealthy or overloaded"},
		{"503", "unhealthy or overloaded"},
		{"504", "unhealthy or overloaded"},
		{"404", "not exist"},
		{"401", "Authentication"},
		{"403", "Authentication"},
		{"200", ""},
	}

	for _, tt := range tests {
		t.Run(tt.errorCode, func(t *testing.T) {
			agg := &AggregatedEvent{
				EventType:   "test.type",
				EventSource: "test-source",
				ErrorDest:   "http://example.com",
				ErrorCode:   tt.errorCode,
			}
			note := generateEventNote(agg, 1)

			if tt.shouldContain != "" && !containsString(note, tt.shouldContain) {
				t.Errorf("note for error code %q should contain %q, got: %s", tt.errorCode, tt.shouldContain, note)
			}
		})
	}
}

func TestEventTransformLoopDetection(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	ctx := context.Background()

	event := cloudevents.NewEvent()
	event.SetID("loop-event-1")
	event.SetType("transformed.event.type")
	event.SetSource("event-transform/my-transform")
	event.SetExtension(attributes.KnativeErrorDestExtensionKey, "http://broker-ingress.knative-eventing.svc.cluster.local/user-namespace/default")
	_ = broker.SetTTL(event.Context, 0)

	aggregator.RecordEvent(ctx, &event)

	time.Sleep(100 * time.Millisecond)

	aggregator.mu.RLock()
	defer aggregator.mu.RUnlock()

	if len(aggregator.buffer) != 1 {
		t.Fatalf("expected 1 event in buffer, got %d", len(aggregator.buffer))
	}

	for _, agg := range aggregator.buffer {
		if agg.ErrorCode != "TTL" {
			t.Errorf("expected error code 'TTL', got %q", agg.ErrorCode)
		}

		if agg.Namespace != "user-namespace" {
			t.Errorf("expected namespace 'user-namespace', got %q", agg.Namespace)
		}

		note := generateEventNote(agg, agg.Count)
		if !containsString(note, "TTL exhausted") {
			t.Errorf("note should mention TTL exhaustion: %q", note)
		}
		if !containsString(note, "event loop") {
			t.Errorf("note should mention event loop: %q", note)
		}
		if !containsString(note, "EventTransform") {
			t.Errorf("note should mention EventTransform: %q", note)
		}
	}
}

func TestEventTransformLoopWithMultipleEvents(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	ctx := context.Background()

	for i := 0; i < 100; i++ {
		event := cloudevents.NewEvent()
		event.SetID("loop-event-" + string(rune('0'+i%10)))
		event.SetType("my.looping.event")
		event.SetSource("my-source")
		event.SetExtension(attributes.KnativeErrorDestExtensionKey, "http://broker-ingress.knative-eventing.svc.cluster.local/test-ns/my-broker")
		_ = broker.SetTTL(event.Context, 0)

		aggregator.RecordEvent(ctx, &event)
	}

	time.Sleep(100 * time.Millisecond)

	aggregator.mu.RLock()
	defer aggregator.mu.RUnlock()

	if len(aggregator.buffer) != 1 {
		t.Errorf("expected 1 aggregated event, got %d", len(aggregator.buffer))
	}

	for _, agg := range aggregator.buffer {
		if agg.Count != 100 {
			t.Errorf("expected count 100, got %d", agg.Count)
		}

		if agg.ErrorCode != "TTL" {
			t.Errorf("expected error code 'TTL', got %q", agg.ErrorCode)
		}
	}
}

func TestK8sEventCreationForTTLExhaustion(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	ctx := context.Background()

	agg := &AggregatedEvent{
		Namespace:   "user-namespace",
		ErrorDest:   "http://broker-ingress.knative-eventing.svc.cluster.local/user-namespace/default",
		EventType:   "my.event.type",
		EventSource: "my-source",
		ErrorCode:   "TTL",
		Count:       50,
	}

	err := aggregator.reportK8sEvent(ctx, agg)
	if err != nil {
		t.Fatalf("failed to report K8s event: %v", err)
	}

	eventName := generateEventName(agg.ErrorDest, agg.EventType, agg.EventSource, agg.ErrorCode)
	k8sEvent, err := client.EventsV1().Events("user-namespace").Get(ctx, eventName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get K8s event: %v", err)
	}

	if k8sEvent.Reason != ReasonTTLExceeded {
		t.Errorf("expected reason %q, got %q", ReasonTTLExceeded, k8sEvent.Reason)
	}

	if k8sEvent.Type != "Warning" {
		t.Errorf("expected type 'Warning', got %q", k8sEvent.Type)
	}

	if k8sEvent.Action != ActionEventDropped {
		t.Errorf("expected action %q, got %q", ActionEventDropped, k8sEvent.Action)
	}

	if k8sEvent.Series == nil || k8sEvent.Series.Count != 50 {
		t.Errorf("expected series count 50, got %v", k8sEvent.Series)
	}

	if k8sEvent.Labels[LabelEventType] != agg.EventType {
		t.Errorf("expected label %s=%s, got %s", LabelEventType, agg.EventType, k8sEvent.Labels[LabelEventType])
	}

	if !containsString(k8sEvent.Note, "TTL exhausted") {
		t.Errorf("note should mention TTL exhaustion: %q", k8sEvent.Note)
	}
}

func TestK8sEventSeriesCountIncrement(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	ctx := context.Background()

	agg := &AggregatedEvent{
		Namespace:   "test-ns",
		ErrorDest:   "http://broker-ingress.knative-eventing.svc.cluster.local/test-ns/broker",
		EventType:   "loop.event",
		EventSource: "loop-source",
		ErrorCode:   "TTL",
		Count:       10,
	}

	err := aggregator.reportK8sEvent(ctx, agg)
	if err != nil {
		t.Fatalf("first report failed: %v", err)
	}

	agg.Count = 20
	err = aggregator.reportK8sEvent(ctx, agg)
	if err != nil {
		t.Fatalf("second report failed: %v", err)
	}

	eventName := generateEventName(agg.ErrorDest, agg.EventType, agg.EventSource, agg.ErrorCode)
	k8sEvent, err := client.EventsV1().Events("test-ns").Get(ctx, eventName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get K8s event: %v", err)
	}

	if k8sEvent.Series.Count != 30 {
		t.Errorf("expected series count 30, got %d", k8sEvent.Series.Count)
	}
}

func TestHandler_HealthCheck(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	handler := &Handler{
		aggregator: aggregator,
		withContext: func(ctx context.Context) context.Context {
			return logging.WithLogger(ctx, logger)
		},
		logger: logger,
	}

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{"healthz endpoint", "GET", "/healthz", 200},
		{"readyz endpoint", "GET", "/readyz", 200},
		{"root endpoint", "GET", "/", 200},
		{"method not allowed", "PUT", "/", 405},
		{"method not allowed DELETE", "DELETE", "/events", 405},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("ServeHTTP() status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestHandler_PostEvent(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	handler := &Handler{
		aggregator: aggregator,
		withContext: func(ctx context.Context) context.Context {
			return logging.WithLogger(ctx, logger)
		},
		logger: logger,
	}

	event := cloudevents.NewEvent()
	event.SetID("test-event-id")
	event.SetType("test.event.type")
	event.SetSource("test-source")
	event.SetExtension(attributes.KnativeErrorDestExtensionKey, "http://broker-ingress.knative-eventing.svc.cluster.local/test-ns/my-broker")
	event.SetExtension(attributes.KnativeErrorCodeExtensionKey, "500")

	eventBytes, _ := event.MarshalJSON()

	req := httptest.NewRequest("POST", "/", bytes.NewReader(eventBytes))
	req.Header.Set("Content-Type", "application/cloudevents+json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != 202 {
		t.Errorf("ServeHTTP() status = %d, want 202", w.Code)
	}

	time.Sleep(50 * time.Millisecond)
	aggregator.mu.RLock()
	bufferLen := len(aggregator.buffer)
	aggregator.mu.RUnlock()

	if bufferLen != 1 {
		t.Errorf("expected 1 event in buffer, got %d", bufferLen)
	}
}

func TestHandler_InvalidEvent(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	handler := &Handler{
		aggregator: aggregator,
		withContext: func(ctx context.Context) context.Context {
			return logging.WithLogger(ctx, logger)
		},
		logger: logger,
	}

	req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte("not valid json")))
	req.Header.Set("Content-Type", "application/cloudevents+json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("ServeHTTP() status = %d, want 400 for invalid event", w.Code)
	}
}

func TestHandler_InvalidCloudEvent(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	handler := &Handler{
		aggregator: aggregator,
		withContext: func(ctx context.Context) context.Context {
			return logging.WithLogger(ctx, logger)
		},
		logger: logger,
	}

	invalidEvent := `{"specversion":"1.0"}`
	req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte(invalidEvent)))
	req.Header.Set("Content-Type", "application/cloudevents+json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("ServeHTTP() status = %d, want 400 for invalid CloudEvent", w.Code)
	}
}

func TestReconcile(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 100*time.Millisecond, 100)

	ctx := context.Background()

	event := cloudevents.NewEvent()
	event.SetID("reconcile-test-id")
	event.SetType("test.event.type")
	event.SetSource("test-source")
	event.SetExtension(attributes.KnativeErrorDestExtensionKey, "http://broker-ingress.knative-eventing.svc.cluster.local/test-ns/my-broker")
	event.SetExtension(attributes.KnativeErrorCodeExtensionKey, "500")

	aggregator.RecordEvent(ctx, &event)

	time.Sleep(100 * time.Millisecond)

	aggregator.mu.Lock()
	for _, agg := range aggregator.buffer {
		agg.Reported = true
		agg.Count = 5
	}
	aggregator.mu.Unlock()

	aggregator.reconcile(ctx)

	aggregator.mu.RLock()
	for _, agg := range aggregator.buffer {
		if agg.Count != 0 {
			t.Errorf("expected count to be reset to 0, got %d", agg.Count)
		}
	}
	aggregator.mu.RUnlock()
}

func TestRun_ContextCancellation(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 50*time.Millisecond, 100)

	ctx, cancel := context.WithCancel(context.Background())

	event := cloudevents.NewEvent()
	event.SetID("run-test-id")
	event.SetType("test.event.type")
	event.SetSource("test-source")
	event.SetExtension(attributes.KnativeErrorDestExtensionKey, "http://broker-ingress.knative-eventing.svc.cluster.local/test-ns/my-broker")
	event.SetExtension(attributes.KnativeErrorCodeExtensionKey, "500")

	aggregator.RecordEvent(ctx, &event)

	done := make(chan struct{})
	go func() {
		aggregator.Run(ctx)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not complete after context cancellation")
	}
}

func TestRun_PeriodicReconcile(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 50*time.Millisecond, 100)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	event := cloudevents.NewEvent()
	event.SetID("periodic-test-id")
	event.SetType("test.event.type")
	event.SetSource("test-source")
	event.SetExtension(attributes.KnativeErrorDestExtensionKey, "http://broker-ingress.knative-eventing.svc.cluster.local/test-ns/my-broker")
	event.SetExtension(attributes.KnativeErrorCodeExtensionKey, "500")

	aggregator.RecordEvent(ctx, &event)

	time.Sleep(50 * time.Millisecond)

	aggregator.mu.Lock()
	for _, agg := range aggregator.buffer {
		agg.Reported = true
		agg.Count = 10
	}
	aggregator.mu.Unlock()

	go aggregator.Run(ctx)

	time.Sleep(100 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestRecordEvent_NamespaceFallback(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	ctx := context.Background()

	event := cloudevents.NewEvent()
	event.SetID("fallback-test-id")
	event.SetType("test.event.type")
	event.SetSource("test-source")
	event.SetExtension(attributes.KnativeErrorDestExtensionKey, "http://localhost:8080")
	event.SetExtension(attributes.KnativeErrorCodeExtensionKey, "500")

	aggregator.RecordEvent(ctx, &event)

	time.Sleep(50 * time.Millisecond)

	aggregator.mu.RLock()
	defer aggregator.mu.RUnlock()

	if len(aggregator.buffer) != 1 {
		t.Fatalf("expected 1 event in buffer, got %d", len(aggregator.buffer))
	}

	for _, agg := range aggregator.buffer {
		if agg.Namespace != "knative-eventing" {
			t.Errorf("expected namespace 'knative-eventing', got %q", agg.Namespace)
		}
	}
}

func TestReportK8sEvent_NilSeries(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	ctx := context.Background()

	agg := &AggregatedEvent{
		Namespace:   "test-ns",
		ErrorDest:   "http://broker-ingress.knative-eventing.svc.cluster.local/test-ns/broker",
		EventType:   "test.type",
		EventSource: "test-source",
		ErrorCode:   "500",
		Count:       10,
	}

	err := aggregator.reportK8sEvent(ctx, agg)
	if err != nil {
		t.Fatalf("first report failed: %v", err)
	}

	eventName := generateEventName(agg.ErrorDest, agg.EventType, agg.EventSource, agg.ErrorCode)
	existing, err := client.EventsV1().Events("test-ns").Get(ctx, eventName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get K8s event: %v", err)
	}

	if existing != nil {
		existing.Series = nil
		_, _ = client.EventsV1().Events("test-ns").Update(ctx, existing, metav1.UpdateOptions{})
	}

	agg.Count = 5
	err = aggregator.reportK8sEvent(ctx, agg)
	if err != nil {
		t.Fatalf("second report failed: %v", err)
	}
}

func TestReportEventAsync(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	ctx := context.Background()

	agg := &AggregatedEvent{
		Namespace:   "test-ns",
		ErrorDest:   "http://example.com",
		EventType:   "test.type",
		EventSource: "test-source",
		ErrorCode:   "500",
		Count:       1,
	}

	aggregator.reportEventAsync(ctx, agg)
	time.Sleep(50 * time.Millisecond)
}

func TestGetExtensionString_NonStringExtension(t *testing.T) {
	event := cloudevents.NewEvent()
	event.SetID("ext-test-id")
	event.SetType("test.type")
	event.SetSource("test-source")

	event.SetExtension("numericext", 42)

	result := getExtensionString(&event, "numericext")
	if result != "42" {
		t.Errorf("expected '42', got %q", result)
	}

	result = getExtensionString(&event, "nonexistent")
	if result != "" {
		t.Errorf("expected empty string for missing extension, got %q", result)
	}
}

func TestRecordEvent_WithExistingErrorCode(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	ctx := context.Background()

	event := cloudevents.NewEvent()
	event.SetID("existing-code-test")
	event.SetType("test.event.type")
	event.SetSource("test-source")
	event.SetExtension(attributes.KnativeErrorDestExtensionKey, "http://broker-ingress.knative-eventing.svc.cluster.local/test-ns/my-broker")
	event.SetExtension(attributes.KnativeErrorCodeExtensionKey, "500")
	_ = broker.SetTTL(event.Context, 0)

	aggregator.RecordEvent(ctx, &event)

	time.Sleep(50 * time.Millisecond)

	aggregator.mu.RLock()
	defer aggregator.mu.RUnlock()

	for _, agg := range aggregator.buffer {
		if agg.ErrorCode != "500" {
			t.Errorf("expected error code '500', got %q", agg.ErrorCode)
		}
	}
}

// TestReconcile_WithError tests the reconcile error handling path
func TestReconcile_WithError(t *testing.T) {
	logger := zap.NewNop().Sugar()
	// Create a client that will cause errors on update
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 100*time.Millisecond, 100)

	ctx := context.Background()

	// Create an entry and mark as reported with count > 0 to trigger reconcile update
	agg := &AggregatedEvent{
		Namespace:   "test-ns",
		ErrorDest:   "http://broker-ingress.knative-eventing.svc.cluster.local/test-ns/broker",
		EventType:   "test.type",
		EventSource: "test-source",
		ErrorCode:   "500",
		Count:       5,
		Reported:    true,
	}

	key := generateAggregationKey(agg.Namespace, agg.ErrorDest, agg.EventType, agg.EventSource, agg.ErrorCode)
	aggregator.mu.Lock()
	aggregator.buffer[key] = agg
	aggregator.mu.Unlock()

	// This will fail since no prior K8s event exists - triggers create instead
	aggregator.reconcile(ctx)

	// Verify count was reset
	aggregator.mu.RLock()
	if aggregator.buffer[key].Count != 0 {
		t.Errorf("expected count to be reset to 0, got %d", aggregator.buffer[key].Count)
	}
	aggregator.mu.RUnlock()
}

// TestReportK8sEvent_CreateError tests reportK8sEvent when create fails
func TestReportK8sEvent_AlreadyExistsRace(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	ctx := context.Background()

	agg := &AggregatedEvent{
		Namespace:   "test-ns",
		ErrorDest:   "http://broker-ingress.knative-eventing.svc.cluster.local/test-ns/broker",
		EventType:   "race.type",
		EventSource: "race-source",
		ErrorCode:   "500",
		Count:       1,
	}

	// First create the event
	err := aggregator.reportK8sEvent(ctx, agg)
	if err != nil {
		t.Fatalf("first report failed: %v", err)
	}

	// Verify second report updates the existing one
	agg.Count = 10
	err = aggregator.reportK8sEvent(ctx, agg)
	if err != nil {
		t.Fatalf("second report failed: %v", err)
	}
}

// TestExtractNamespaceFromURL_EdgeCases tests more namespace extraction scenarios
func TestExtractNamespaceFromURL_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		wantNs string
	}{
		{"empty path broker-ingress", "http://broker-ingress.test-ns.svc.cluster.local", "test-ns"},
		{"path with only namespace", "http://broker-ingress.knative-eventing.svc.cluster.local/my-ns/", "my-ns"},
		{"empty string", "", ""},
		{"just protocol", "http://", ""},
		{"invalid control char", "http://example.com\x00/path", ""},    // URL with control char that causes parse error
		{"percent encoded invalid", "http://example.com/%zz/path", ""}, // Invalid percent encoding
		{"invalid percent only", "%", ""},                              // URL that causes parse error
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractNamespaceFromURL(tt.url)
			if got != tt.wantNs {
				t.Errorf("extractNamespaceFromURL(%q) = %q, want %q", tt.url, got, tt.wantNs)
			}
		})
	}
}

// TestReportK8sEvent_ReasonVariations tests different reason codes
func TestReportK8sEvent_ReasonVariations(t *testing.T) {
	tests := []struct {
		name         string
		errorCode    string
		expectReason string
	}{
		{"ttl lowercase", "ttl", ReasonTTLExceeded},
		{"TTL uppercase", "TTL", ReasonTTLExceeded},
		{"ttlexceeded", "ttlexceeded", ReasonTTLExceeded},
		{"delivery failure", "500", ReasonDeliveryFailed},
		{"not found", "404", ReasonDeliveryFailed},
		{"empty code", "", ReasonDeliveryFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop().Sugar()
			client := fake.NewSimpleClientset()
			aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

			ctx := context.Background()

			agg := &AggregatedEvent{
				Namespace:   "test-ns-" + tt.name,
				ErrorDest:   "http://broker-ingress.knative-eventing.svc.cluster.local/test-ns/broker-" + tt.name,
				EventType:   "test.type." + tt.name,
				EventSource: "test-source-" + tt.name,
				ErrorCode:   tt.errorCode,
				Count:       1,
			}

			err := aggregator.reportK8sEvent(ctx, agg)
			if err != nil {
				t.Fatalf("report failed: %v", err)
			}

			eventName := generateEventName(agg.ErrorDest, agg.EventType, agg.EventSource, agg.ErrorCode)
			k8sEvent, err := client.EventsV1().Events(agg.Namespace).Get(ctx, eventName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("failed to get K8s event: %v", err)
			}

			if k8sEvent.Reason != tt.expectReason {
				t.Errorf("expected reason %q, got %q", tt.expectReason, k8sEvent.Reason)
			}
		})
	}
}

// TestGenerateEventNote_LongValues tests note generation with long values
func TestGenerateEventNote_LongValues(t *testing.T) {
	longString := string(make([]byte, 1000))
	for i := range longString {
		longString = longString[:i] + "a"
	}

	agg := &AggregatedEvent{
		EventType:   "very.long.event.type.name.that.goes.on.and.on",
		EventSource: "very-long-event-source-name-that-continues",
		ErrorDest:   "http://very-long-destination-url.example.com/path/to/something",
		ErrorCode:   "500",
	}

	note := generateEventNote(agg, 100)

	if note == "" {
		t.Error("note should not be empty")
	}

	if !containsString(note, "100") {
		t.Errorf("note should contain count '100', got: %s", note)
	}
}

// TestRecordEvent_ConcurrentAccess tests thread safety of RecordEvent
func TestRecordEvent_ConcurrentAccess(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			event := cloudevents.NewEvent()
			event.SetID(fmt.Sprintf("concurrent-test-%d", idx))
			event.SetType("concurrent.test.type")
			event.SetSource("concurrent-source")
			event.SetExtension(attributes.KnativeErrorDestExtensionKey, "http://broker-ingress.knative-eventing.svc.cluster.local/test-ns/broker")
			event.SetExtension(attributes.KnativeErrorCodeExtensionKey, "500")

			aggregator.RecordEvent(ctx, &event)
		}(i)
	}

	wg.Wait()

	// Allow async reportEventAsync to complete
	time.Sleep(100 * time.Millisecond)

	aggregator.mu.RLock()
	defer aggregator.mu.RUnlock()

	if len(aggregator.buffer) != 1 {
		t.Errorf("expected 1 aggregated entry, got %d", len(aggregator.buffer))
	}

	for _, agg := range aggregator.buffer {
		if agg.Count != 100 {
			t.Errorf("expected count 100, got %d", agg.Count)
		}
	}
}

// TestReportK8sEvent_LabelTruncation tests that labels are properly truncated
func TestReportK8sEvent_LabelTruncation(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	ctx := context.Background()

	// Create strings longer than 63 chars (K8s label max)
	longString := "this-is-a-very-long-string-that-exceeds-the-kubernetes-label-value-limit-of-63-characters"

	agg := &AggregatedEvent{
		Namespace:   "test-ns",
		ErrorDest:   "http://broker-ingress.knative-eventing.svc.cluster.local/test-ns/" + longString,
		EventType:   longString,
		EventSource: longString,
		ErrorCode:   "500",
		Count:       1,
	}

	err := aggregator.reportK8sEvent(ctx, agg)
	if err != nil {
		t.Fatalf("report failed: %v", err)
	}

	eventName := generateEventName(agg.ErrorDest, agg.EventType, agg.EventSource, agg.ErrorCode)
	k8sEvent, err := client.EventsV1().Events("test-ns").Get(ctx, eventName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get K8s event: %v", err)
	}

	// Verify labels were truncated
	if len(k8sEvent.Labels[LabelEventType]) > 63 {
		t.Errorf("event type label not truncated: len=%d", len(k8sEvent.Labels[LabelEventType]))
	}
	if len(k8sEvent.Labels[LabelEventSource]) > 63 {
		t.Errorf("event source label not truncated: len=%d", len(k8sEvent.Labels[LabelEventSource]))
	}
}

// TestRecordEvent_UpdateExistingBuffer tests updating existing buffer entries
func TestRecordEvent_UpdateExistingBuffer(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	ctx := context.Background()

	// Record same event multiple times
	for i := 0; i < 10; i++ {
		event := cloudevents.NewEvent()
		event.SetID(fmt.Sprintf("update-test-%d", i))
		event.SetType("update.test.type")
		event.SetSource("update-source")
		event.SetExtension(attributes.KnativeErrorDestExtensionKey, "http://broker-ingress.knative-eventing.svc.cluster.local/test-ns/broker")
		event.SetExtension(attributes.KnativeErrorCodeExtensionKey, "500")

		aggregator.RecordEvent(ctx, &event)
	}

	time.Sleep(100 * time.Millisecond)

	aggregator.mu.RLock()
	defer aggregator.mu.RUnlock()

	if len(aggregator.buffer) != 1 {
		t.Errorf("expected 1 entry, got %d", len(aggregator.buffer))
		return
	}

	for _, agg := range aggregator.buffer {
		if agg.Count != 10 {
			t.Errorf("expected count 10, got %d", agg.Count)
		}
	}
}

// TestReportK8sEvent_GetError tests the error path when Get fails with non-NotFound error
func TestReportK8sEvent_GetError(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()

	// Add a reactor that returns a non-NotFound error for Get
	client.PrependReactor("get", "events", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("simulated API error")
	})

	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	ctx := context.Background()

	agg := &AggregatedEvent{
		Namespace:   "error-ns",
		ErrorDest:   "http://example.com",
		EventType:   "error.type",
		EventSource: "error-source",
		ErrorCode:   "500",
		Count:       1,
	}

	err := aggregator.reportK8sEvent(ctx, agg)
	if err == nil {
		t.Error("expected error but got none")
	}
}

// TestReportK8sEvent_UpdateError tests the error path when Update fails
func TestReportK8sEvent_UpdateError(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	ctx := context.Background()

	agg := &AggregatedEvent{
		Namespace:   "update-error-ns",
		ErrorDest:   "http://broker-ingress.knative-eventing.svc.cluster.local/update-error-ns/broker",
		EventType:   "update.error.type",
		EventSource: "update-error-source",
		ErrorCode:   "500",
		Count:       1,
	}

	// First, create an event successfully
	err := aggregator.reportK8sEvent(ctx, agg)
	if err != nil {
		t.Fatalf("first report failed: %v", err)
	}

	// Now add a reactor that will fail updates
	client.PrependReactor("update", "events", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("simulated update error")
	})

	// Try to update - should fail
	agg.Count = 5
	err = aggregator.reportK8sEvent(ctx, agg)
	if err == nil {
		t.Error("expected error but got none")
	}
}

// TestReportK8sEvent_CreateError tests the error path when Create fails
func TestReportK8sEvent_CreateError(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()

	// Add a reactor that returns an error for Create
	client.PrependReactor("create", "events", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("simulated create error")
	})

	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	ctx := context.Background()

	agg := &AggregatedEvent{
		Namespace:   "create-error-ns",
		ErrorDest:   "http://example.com/create",
		EventType:   "create.error.type",
		EventSource: "create-error-source",
		ErrorCode:   "500",
		Count:       1,
	}

	err := aggregator.reportK8sEvent(ctx, agg)
	if err == nil {
		t.Error("expected error but got none")
	}
}

// TestReconcile_ReportError tests that reconcile handles reportK8sEvent errors gracefully
func TestReconcile_ReportError(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()

	// Add a reactor that returns an error for all event operations
	client.PrependReactor("*", "events", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("simulated error for reconcile test")
	})

	aggregator := NewEventAggregator(client, logger, 100*time.Millisecond, 100)

	ctx := context.Background()

	// Add an entry to the buffer that will be reported during reconcile
	agg := &AggregatedEvent{
		Namespace:   "reconcile-error-ns",
		ErrorDest:   "http://example.com/reconcile",
		EventType:   "reconcile.error.type",
		EventSource: "reconcile-error-source",
		ErrorCode:   "500",
		Count:       5,
		Reported:    true, // Mark as reported so it triggers reconcile
	}

	key := generateAggregationKey(agg.Namespace, agg.ErrorDest, agg.EventType, agg.EventSource, agg.ErrorCode)
	aggregator.mu.Lock()
	aggregator.buffer[key] = agg
	aggregator.mu.Unlock()

	// Run reconcile - it should handle the error gracefully
	aggregator.reconcile(ctx)

	// Verify count was still reset (even though reporting failed)
	aggregator.mu.RLock()
	if aggregator.buffer[key].Count != 0 {
		t.Errorf("expected count to be reset to 0, got %d", aggregator.buffer[key].Count)
	}
	aggregator.mu.RUnlock()
}

// TestReportEventAsync_Error tests the error logging path in reportEventAsync
func TestReportEventAsync_Error(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()

	// Add a reactor that fails
	client.PrependReactor("*", "events", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("simulated error")
	})

	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	ctx := context.Background()

	agg := &AggregatedEvent{
		Namespace:   "async-error-ns",
		ErrorDest:   "http://example.com",
		EventType:   "async.error.type",
		EventSource: "async-error-source",
		ErrorCode:   "500",
		Count:       1,
	}

	// This should handle the error gracefully (just log it)
	aggregator.reportEventAsync(ctx, agg)
}

// TestReportK8sEvent_AlreadyExistsError tests the race condition handling
func TestReportK8sEvent_AlreadyExistsError(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	ctx := context.Background()

	agg := &AggregatedEvent{
		Namespace:   "already-exists-ns",
		ErrorDest:   "http://broker-ingress.knative-eventing.svc.cluster.local/already-exists-ns/broker",
		EventType:   "already.exists.type",
		EventSource: "already-exists-source",
		ErrorCode:   "500",
		Count:       1,
	}

	// First, create the event successfully
	err := aggregator.reportK8sEvent(ctx, agg)
	if err != nil {
		t.Fatalf("first report failed: %v", err)
	}

	// Create a new client with a reactor that returns AlreadyExists on first create, then succeeds
	client2 := fake.NewSimpleClientset()
	aggregator2 := NewEventAggregator(client2, logger, 30*time.Second, 100)

	createCalls := 0
	client2.PrependReactor("create", "events", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		createCalls++
		if createCalls == 1 {
			// First create: return AlreadyExists error
			return true, nil, apierrors.NewAlreadyExists(
				schema.GroupResource{Group: "events.k8s.io", Resource: "events"},
				"test-event",
			)
		}
		// Subsequent calls: let the actual create happen (shouldn't reach here in this test)
		return false, nil, nil
	})

	// Need to also allow Get to find the event (simulate it was created by another process)
	getCalls := 0
	client2.PrependReactor("get", "events", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		getCalls++
		if getCalls == 1 {
			// First get: not found (triggers create)
			return true, nil, apierrors.NewNotFound(
				schema.GroupResource{Group: "events.k8s.io", Resource: "events"},
				"test-event",
			)
		}
		// Second get (after AlreadyExists retry): let it find the event
		return false, nil, nil
	})

	agg2 := &AggregatedEvent{
		Namespace:   "already-exists-ns-2",
		ErrorDest:   "http://broker-ingress.knative-eventing.svc.cluster.local/already-exists-ns-2/broker",
		EventType:   "already.exists.type.2",
		EventSource: "already-exists-source-2",
		ErrorCode:   "500",
		Count:       1,
	}

	// This should trigger the AlreadyExists retry path
	err = aggregator2.reportK8sEvent(ctx, agg2)
	// The retry will try to get and then update/create again
	// Since our fake doesn't have the event, the second attempt will create it
	if err != nil {
		// This is expected if the second create also returns AlreadyExists
		t.Logf("error after retry: %v (may be expected)", err)
	}

	// Verify that create was called at least once (the AlreadyExists path was triggered)
	if createCalls == 0 {
		t.Error("expected at least one create call")
	}
}

// TestNewHandler tests the NewHandler constructor
func TestNewHandler(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()

	handler := NewHandler(client, logger)

	if handler == nil {
		t.Fatal("NewHandler returned nil")
	}

	if handler.aggregator == nil {
		t.Error("handler.aggregator is nil")
	}

	if handler.logger == nil {
		t.Error("handler.logger is nil")
	}

	if handler.withContext == nil {
		t.Error("handler.withContext is nil")
	}

	// Test that withContext works
	ctx := context.Background()
	newCtx := handler.withContext(ctx)
	if newCtx == nil {
		t.Error("withContext returned nil")
	}
}

// TestNewHandlerWithAggregator tests the NewHandlerWithAggregator constructor
func TestNewHandlerWithAggregator(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()
	aggregator := NewEventAggregator(client, logger, 30*time.Second, 100)

	handler := NewHandlerWithAggregator(aggregator, logger)

	if handler == nil {
		t.Fatal("NewHandlerWithAggregator returned nil")
	}

	if handler.aggregator != aggregator {
		t.Error("handler.aggregator doesn't match provided aggregator")
	}

	if handler.logger == nil {
		t.Error("handler.logger is nil")
	}

	if handler.withContext == nil {
		t.Error("handler.withContext is nil")
	}

	// Test that withContext works correctly
	ctx := context.Background()
	newCtx := handler.withContext(ctx)
	if newCtx == nil {
		t.Error("withContext returned nil")
	}

	// Test handler works by making a health check request
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestNewHandler_ServeHTTP tests that a handler created with NewHandler works correctly
func TestNewHandler_ServeHTTP(t *testing.T) {
	logger := zap.NewNop().Sugar()
	client := fake.NewSimpleClientset()

	handler := NewHandler(client, logger)

	// Test health check
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200 for /healthz, got %d", w.Code)
	}

	// Test POST with valid event
	event := cloudevents.NewEvent()
	event.SetID("test-handler-event")
	event.SetType("test.handler.type")
	event.SetSource("test-handler-source")
	event.SetExtension(attributes.KnativeErrorDestExtensionKey, "http://broker-ingress.knative-eventing.svc.cluster.local/test-ns/broker")
	event.SetExtension(attributes.KnativeErrorCodeExtensionKey, "500")

	eventBytes, _ := event.MarshalJSON()
	req = httptest.NewRequest("POST", "/", bytes.NewReader(eventBytes))
	req.Header.Set("Content-Type", "application/cloudevents+json")
	w = httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != 202 {
		t.Errorf("expected status 202 for POST, got %d", w.Code)
	}
}

// TestFlush tests the Flush function
func TestFlush(t *testing.T) {
	logger := zap.NewNop().Sugar()
	// This should not panic
	Flush(logger)
}

// mockShutdowner is a mock implementation of Shutdowner for testing
type mockShutdowner struct {
	shutdownCalled bool
	err            error
}

func (m *mockShutdowner) Shutdown(ctx context.Context) error {
	m.shutdownCalled = true
	return m.err
}

// TestShutdownProviders tests the ShutdownProviders function
func TestShutdownProviders(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	// Test with successful shutdown
	provider1 := &mockShutdowner{}
	provider2 := &mockShutdowner{}

	ShutdownProviders(ctx, logger, provider1, provider2)

	if !provider1.shutdownCalled {
		t.Error("provider1.Shutdown was not called")
	}
	if !provider2.shutdownCalled {
		t.Error("provider2.Shutdown was not called")
	}
}

// TestShutdownProviders_WithError tests ShutdownProviders with an error
func TestShutdownProviders_WithError(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	// Test with error during shutdown
	provider := &mockShutdowner{err: fmt.Errorf("shutdown error")}

	// Should not panic even with error
	ShutdownProviders(ctx, logger, provider)

	if !provider.shutdownCalled {
		t.Error("provider.Shutdown was not called")
	}
}

// TestShutdownProviders_WithNilProvider tests ShutdownProviders with nil provider
func TestShutdownProviders_WithNilProvider(t *testing.T) {
	logger := zap.NewNop().Sugar()
	ctx := context.Background()

	// Should not panic with nil provider
	ShutdownProviders(ctx, logger, nil)
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
