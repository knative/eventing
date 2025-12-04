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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"knative.dev/eventing/pkg/broker"
	"knative.dev/eventing/pkg/kncloudevents/attributes"
)

const (
	// K8s event labels
	LabelEventType   = "eventing.knative.dev/event-type"
	LabelEventSource = "eventing.knative.dev/event-source"
	LabelErrorDest   = "eventing.knative.dev/error-dest"

	// K8s event action and reasons
	ActionEventDropped   = "EventDropped"
	ReasonDeliveryFailed = "DeliveryFailed"
	ReasonTTLExceeded    = "TTLExceeded"
	ReportingController  = "knative.dev/eventing/k8s-event-reporter"

	// Event name prefix
	EventNamePrefix = "knative-eventing-dlq"
)

// AggregatedEvent holds aggregated information about dropped events
type AggregatedEvent struct {
	// Key fields for aggregation
	Namespace   string
	ErrorDest   string
	EventType   string
	EventSource string
	ErrorCode   string

	// First occurrence details
	FirstSeenTime time.Time
	FirstEventID  string

	// Aggregated counts
	Count int32

	// Last occurrence
	LastSeenTime time.Time

	// Whether this has been reported at least once
	Reported bool
}

// EventAggregator aggregates dead-letter events and periodically reports them as K8s events
type EventAggregator struct {
	client        kubernetes.Interface
	logger        *zap.SugaredLogger
	window        time.Duration
	maxBufferSize int

	mu     sync.RWMutex
	buffer map[string]*AggregatedEvent
}

// NewEventAggregator creates a new EventAggregator
func NewEventAggregator(client kubernetes.Interface, logger *zap.SugaredLogger, window time.Duration, maxBufferSize int) *EventAggregator {
	return &EventAggregator{
		client:        client,
		logger:        logger,
		window:        window,
		maxBufferSize: maxBufferSize,
		buffer:        make(map[string]*AggregatedEvent),
	}
}

// RecordEvent records a dead-letter event for aggregation
func (a *EventAggregator) RecordEvent(ctx context.Context, event *cloudevents.Event) {
	// Extract error destination from CloudEvents extension
	errorDest := getExtensionString(event, attributes.KnativeErrorDestExtensionKey)
	errorCode := getExtensionString(event, attributes.KnativeErrorCodeExtensionKey)

	if errorDest == "" {
		a.logger.Warnw("Dead-letter event missing error destination extension",
			"eventID", event.ID(),
			"eventType", event.Type(),
			"eventSource", event.Source())
		return
	}

	// Check if this might be a TTL exhaustion (event loop detection)
	// TTL <= 0 indicates the event has looped through the broker too many times
	ttl, err := broker.GetTTL(event.Context)
	isTTLExhausted := err == nil && ttl <= 0

	if isTTLExhausted && errorCode == "" {
		errorCode = "TTL"
	}

	// Parse the error destination to extract namespace (best effort)
	namespace := extractNamespaceFromURL(errorDest)
	if namespace == "" {
		// Default to system namespace if we can't determine it
		namespace = "knative-eventing"
	}

	now := time.Now()
	key := generateAggregationKey(namespace, errorDest, event.Type(), event.Source(), errorCode)

	a.mu.Lock()
	defer a.mu.Unlock()

	// Check buffer size limit
	if len(a.buffer) >= a.maxBufferSize {
		if _, exists := a.buffer[key]; !exists {
			a.logger.Warnw("Event aggregation buffer full, dropping event",
				"bufferSize", len(a.buffer),
				"maxSize", a.maxBufferSize)
			return
		}
	}

	if existing, ok := a.buffer[key]; ok {
		// Update existing aggregation
		existing.Count++
		existing.LastSeenTime = now
	} else {
		// Create new aggregation entry
		a.buffer[key] = &AggregatedEvent{
			Namespace:     namespace,
			ErrorDest:     errorDest,
			EventType:     event.Type(),
			EventSource:   event.Source(),
			ErrorCode:     errorCode,
			FirstSeenTime: now,
			FirstEventID:  event.ID(),
			Count:         1,
			LastSeenTime:  now,
			Reported:      false,
		}

		// Report immediately on first occurrence
		go a.reportEventAsync(ctx, a.buffer[key])
	}
}

// Run starts the periodic reconciliation loop
func (a *EventAggregator) Run(ctx context.Context) {
	ticker := time.NewTicker(a.window)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Final flush before shutdown
			a.reconcile(ctx)
			return
		case <-ticker.C:
			a.reconcile(ctx)
		}
	}
}

// reconcile processes all buffered events and reports them as K8s events
func (a *EventAggregator) reconcile(ctx context.Context) {
	a.mu.Lock()
	// Copy and reset the buffer
	eventsToReport := make([]*AggregatedEvent, 0, len(a.buffer))
	for key, agg := range a.buffer {
		// Only report if there are new events since last report
		if agg.Reported && agg.Count > 0 {
			eventsToReport = append(eventsToReport, &AggregatedEvent{
				Namespace:     agg.Namespace,
				ErrorDest:     agg.ErrorDest,
				EventType:     agg.EventType,
				EventSource:   agg.EventSource,
				ErrorCode:     agg.ErrorCode,
				FirstSeenTime: agg.FirstSeenTime,
				FirstEventID:  agg.FirstEventID,
				Count:         agg.Count,
				LastSeenTime:  agg.LastSeenTime,
				Reported:      agg.Reported,
			})
		}
		// Reset count but keep the entry for series tracking
		a.buffer[key].Count = 0
		a.buffer[key].Reported = true
	}
	a.mu.Unlock()

	for _, agg := range eventsToReport {
		if err := a.reportK8sEvent(ctx, agg); err != nil {
			a.logger.Errorw("Failed to report K8s event",
				"namespace", agg.Namespace,
				"errorDest", agg.ErrorDest,
				"error", err)
		}
	}
}

// reportEventAsync reports the first occurrence of an event asynchronously
func (a *EventAggregator) reportEventAsync(ctx context.Context, agg *AggregatedEvent) {
	if err := a.reportK8sEvent(ctx, agg); err != nil {
		a.logger.Errorw("Failed to report initial K8s event",
			"namespace", agg.Namespace,
			"errorDest", agg.ErrorDest,
			"error", err)
	}
}

// reportK8sEvent creates or updates a K8s event for the aggregated event data
func (a *EventAggregator) reportK8sEvent(ctx context.Context, agg *AggregatedEvent) error {
	eventName := generateEventName(agg.ErrorDest, agg.EventType, agg.EventSource, agg.ErrorCode)
	now := metav1.NowMicro()

	// Try to get existing event first
	existing, err := a.client.EventsV1().Events(agg.Namespace).Get(ctx, eventName, metav1.GetOptions{})
	if err == nil && existing != nil {
		// Update existing event's series count
		if existing.Series == nil {
			existing.Series = &eventsv1.EventSeries{
				Count:            agg.Count,
				LastObservedTime: now,
			}
		} else {
			existing.Series.Count += agg.Count
			existing.Series.LastObservedTime = now
		}
		existing.Note = generateEventNote(agg, existing.Series.Count)

		_, err = a.client.EventsV1().Events(agg.Namespace).Update(ctx, existing, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update K8s event: %w", err)
		}
		a.logger.Debugw("Updated K8s event",
			"name", eventName,
			"namespace", agg.Namespace,
			"seriesCount", existing.Series.Count)
		return nil
	}

	if !apierrors.IsNotFound(err) && err != nil {
		return fmt.Errorf("failed to get existing K8s event: %w", err)
	}

	// Create new event
	reason := ReasonDeliveryFailed
	if strings.EqualFold(agg.ErrorCode, "TTL") || strings.Contains(strings.ToLower(agg.ErrorCode), "ttl") {
		reason = ReasonTTLExceeded
	}

	// Truncate labels to max length allowed by K8s (63 chars for label values)
	truncatedEventType := truncateString(agg.EventType, 63)
	truncatedEventSource := truncateString(agg.EventSource, 63)
	truncatedErrorDest := truncateString(agg.ErrorDest, 63)

	k8sEvent := &eventsv1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      eventName,
			Namespace: agg.Namespace,
			Labels: map[string]string{
				LabelEventType:   truncatedEventType,
				LabelEventSource: truncatedEventSource,
				LabelErrorDest:   truncatedErrorDest,
			},
		},
		EventTime:           now,
		ReportingController: ReportingController,
		ReportingInstance:   ReportingController,
		Action:              ActionEventDropped,
		Reason:              reason,
		Type:                corev1.EventTypeWarning,
		Note:                generateEventNote(agg, agg.Count),
		Regarding: corev1.ObjectReference{
			APIVersion: "eventing.knative.dev/v1",
			Kind:       "Broker",
			Name:       extractResourceName(agg.ErrorDest),
			Namespace:  agg.Namespace,
		},
		Series: &eventsv1.EventSeries{
			Count:            agg.Count,
			LastObservedTime: now,
		},
	}

	_, err = a.client.EventsV1().Events(agg.Namespace).Create(ctx, k8sEvent, metav1.CreateOptions{})
	if err != nil {
		// If it already exists (race condition), try to update
		if apierrors.IsAlreadyExists(err) {
			return a.reportK8sEvent(ctx, agg)
		}
		return fmt.Errorf("failed to create K8s event: %w", err)
	}

	a.logger.Infow("Created K8s event for dropped CloudEvent",
		"name", eventName,
		"namespace", agg.Namespace,
		"eventType", agg.EventType,
		"eventSource", agg.EventSource,
		"count", agg.Count)

	return nil
}

// Helper functions

func getExtensionString(event *cloudevents.Event, key string) string {
	ext, err := event.Context.GetExtension(key)
	if err != nil {
		return ""
	}
	if s, ok := ext.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", ext)
}

func generateAggregationKey(namespace, errorDest, eventType, eventSource, errorCode string) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s", namespace, errorDest, eventType, eventSource, errorCode)
}

func generateEventName(errorDest, eventType, eventSource, errorCode string) string {
	// Create a deterministic name based on the key fields
	data := fmt.Sprintf("%s|%s|%s|%s", errorDest, eventType, eventSource, errorCode)
	hash := sha256.Sum256([]byte(data))
	shortHash := hex.EncodeToString(hash[:])[:16]
	return fmt.Sprintf("%s.%s", EventNamePrefix, shortHash)
}

func generateEventNote(agg *AggregatedEvent, totalCount int32) string {
	note := fmt.Sprintf("Event of type %q and source %q failed delivery to %q. Error code: %s. Series count: %d.",
		agg.EventType, agg.EventSource, agg.ErrorDest, agg.ErrorCode, totalCount)

	// Add hints based on error code
	if agg.ErrorCode != "" {
		switch {
		case strings.EqualFold(agg.ErrorCode, "TTL") || strings.Contains(strings.ToLower(agg.ErrorCode), "ttl"):
			note += " Event TTL exhausted, indicating a probable event loop. Check your Trigger filters and EventTransform configurations."
		case agg.ErrorCode == "500" || agg.ErrorCode == "502" || agg.ErrorCode == "503" || agg.ErrorCode == "504":
			note += " The subscriber service may be unhealthy or overloaded."
		case agg.ErrorCode == "404":
			note += " The subscriber endpoint may not exist or be misconfigured."
		case agg.ErrorCode == "401" || agg.ErrorCode == "403":
			note += " Authentication or authorization failure. Check OIDC configuration."
		}
	}

	return note
}

func extractNamespaceFromURL(urlStr string) string {
	// Try to parse the URL to extract namespace from the path
	// Expected format: http://broker-ingress.knative-eventing.svc.cluster.local/{namespace}/{broker-name}
	// Or: http://{service}.{namespace}.svc.cluster.local/...
	u, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}

	// Check if URL has a valid scheme (must be http or https)
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}

	// Check if this is a broker ingress URL pattern by looking at the host
	hostParts := strings.Split(u.Host, ".")

	// For broker-ingress pattern: broker-ingress.knative-eventing.svc.cluster.local
	// The path contains /{namespace}/{broker-name}
	if len(hostParts) >= 2 && hostParts[0] == "broker-ingress" {
		// Try path-based namespace extraction (broker ingress pattern)
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) >= 1 && parts[0] != "" {
			return parts[0]
		}
	}

	// For service pattern: {service}.{namespace}.svc.cluster.local
	// Check if this looks like a Kubernetes service URL
	if len(hostParts) >= 4 && hostParts[2] == "svc" {
		return hostParts[1]
	}

	// Fallback: try hostname-based namespace extraction for 2+ part hostnames
	if len(hostParts) >= 2 {
		return hostParts[1]
	}

	return ""
}

func extractResourceName(urlStr string) string {
	// Try to extract resource name from URL
	// Expected format: http://broker-ingress.knative-eventing.svc.cluster.local/{namespace}/{broker-name}
	u, err := url.Parse(urlStr)
	if err != nil {
		return "unknown"
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) >= 2 {
		return parts[1]
	}

	return "unknown"
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
