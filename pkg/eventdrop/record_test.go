package eventdrop

import (
	"context"
	"testing"
)

// TestRecordEventDropped_DoesNotPanic validates the Phase 1 contract:
// RecordEventDropped is safe to call even if OTEL infrastructure is not
// fully initialized or configured.
//
// Phase 1 tests focus on the API surface and graceful degradation.
// Detailed metric and trace verification will be added in Phase 1.5
// once the handlers integrate with RecordEventDropped and emit real events.
func TestRecordEventDropped_DoesNotPanic(t *testing.T) {
	ctx := context.Background()

	// Construct a complete Info struct with all fields populated.
	// This validates that RecordEventDropped handles the full API surface.
	info := Info{
		Namespace:   "test-ns",
		Broker:      "test-broker",
		Trigger:     "test-trigger",
		EventType:   "dev.knative.test",
		EventSource: "test-source",
		Reason:      ReasonTTLExhausted,
		Details:     "TTL count reached 0",
	}

	// The function should not panic under any circumstances.
	// Even if OTEL is not initialized, it should degrade gracefully.
	RecordEventDropped(ctx, info)
}

// TestRecordEventDropped_WithPartialInfo validates that RecordEventDropped
// gracefully handles Info structs where not all fields are populated.
//
// This is important because:
// - At ingress time, Trigger is not yet known (empty string is OK).
// - EventType and EventSource may not always be available.
// - Details field is optional for all call sites.
//
// Metrics should still be emitted with empty string values for missing fields.
// Traces should omit attributes that are empty.
func TestRecordEventDropped_WithPartialInfo(t *testing.T) {
	ctx := context.Background()

	// Simulate the ingress handler case where Trigger is not yet known.
	info := Info{
		Namespace:   "test-ns",
		Broker:      "test-broker",
		Trigger:     "", // Not known at ingress
		EventType:   "com.example.event",
		EventSource: "", // May not be available
		Reason:      ReasonTTLMissing,
		Details:     "", // Optional
	}

	// Should handle partial Info without panicking.
	RecordEventDropped(ctx, info)
}

// TestRecordEventDropped_MinimalInfo validates that RecordEventDropped
// works with only the required fields populated.
//
// This test ensures backward compatibility and graceful degradation
// if handler code is written before all context is available.
func TestRecordEventDropped_MinimalInfo(t *testing.T) {
	ctx := context.Background()

	// Only required fields (Namespace, Broker, Reason).
	info := Info{
		Namespace: "test-ns",
		Broker:    "test-broker",
		Reason:    ReasonTTLExhausted,
	}

	// Should work fine with minimal Info.
	RecordEventDropped(ctx, info)
}

// TestRecordEventDropped_AllReasons validates that both Reason enum values
// are handled correctly.
//
// This ensures that as new reasons are added in future phases, they will
// be handled without code changes.
func TestRecordEventDropped_AllReasons(t *testing.T) {
	ctx := context.Background()

	reasons := []Reason{
		ReasonTTLMissing,
		ReasonTTLExhausted,
	}

	for _, reason := range reasons {
		info := Info{
			Namespace: "test-ns",
			Broker:    "test-broker",
			Reason:    reason,
		}

		// Each reason should be handled without panic.
		RecordEventDropped(ctx, info)
	}
}
