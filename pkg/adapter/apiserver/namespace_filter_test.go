package apiserver

import (
	"testing"

	"go.uber.org/zap"

	adaptertest "knative.dev/eventing/pkg/adapter/v2/test"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/eventfilter/subscriptionsapi"
)

// TestNamespaceFiltering tests that events from non-allowed namespaces are dropped
func TestNamespaceFiltering(t *testing.T) {
	ce := adaptertest.NewTestClient()

	// Create delegate with namespace filter for "allowed-ns"
	allowedNs := map[string]struct{}{
		"allowed-ns": {},
	}

	logger := zap.NewExample().Sugar()
	delegate := &resourceDelegate{
		ce:                  ce,
		source:              "unit-test",
		ref:                 false,
		apiServerSourceName: "test-source",
		logger:              logger,
		filter:              subscriptionsapi.NewAllFilter(subscriptionsapi.MaterializeFiltersList(logger.Desugar(), []eventingv1.SubscriptionsAPIFilter{})...),
		allowedNamespaces:   allowedNs,
		filterByNamespace:   true,
	}

	// Test 1: Event from allowed namespace SHOULD be sent
	allowedPod := simplePod("allowed-pod", "allowed-ns")
	err := delegate.Add(allowedPod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(ce.Sent()); got != 1 {
		t.Errorf("Expected 1 event to be sent for allowed namespace, got: %d", got)
	}

	// Reset before next test
	ce.Reset()

	// Test 2: Event from non-allowed namespace should NOT be sent
	deniedPod := simplePod("denied-pod", "denied-ns")
	err = delegate.Add(deniedPod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(ce.Sent()); got != 0 {
		t.Errorf("Expected 0 events to be sent for denied namespace, got: %d", got)
	}
}

// TestNamespaceFilteringDisabled tests that when filtering is disabled, all events are sent
func TestNamespaceFilteringDisabled(t *testing.T) {
	ce := adaptertest.NewTestClient()

	logger := zap.NewExample().Sugar()
	// Create delegate WITHOUT namespace filtering
	delegate := &resourceDelegate{
		ce:                  ce,
		source:              "unit-test",
		ref:                 false,
		apiServerSourceName: "test-source",
		logger:              logger,
		filter:              subscriptionsapi.NewAllFilter(subscriptionsapi.MaterializeFiltersList(logger.Desugar(), []eventingv1.SubscriptionsAPIFilter{})...),
		filterByNamespace:   false, // Filtering disabled
	}

	// Both namespaces should send events
	pod1 := simplePod("pod-1", "ns-1")
	err := delegate.Add(pod1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(ce.Sent()); got != 1 {
		t.Errorf("Expected 1 event after first pod, got: %d", got)
	}

	pod2 := simplePod("pod-2", "ns-2")
	err = delegate.Add(pod2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(ce.Sent()); got != 2 {
		t.Errorf("Expected 2 events after second pod, got: %d", got)
	}
}

// TestNamespaceFilteringMultipleAllowed tests multiple allowed namespaces
func TestNamespaceFilteringMultipleAllowed(t *testing.T) {
	ce := adaptertest.NewTestClient()

	// Create delegate with multiple allowed namespaces
	allowedNs := map[string]struct{}{
		"prod-1": {},
		"prod-2": {},
		"prod-3": {},
	}

	logger := zap.NewExample().Sugar()
	delegate := &resourceDelegate{
		ce:                  ce,
		source:              "unit-test",
		ref:                 false,
		apiServerSourceName: "test-source",
		logger:              logger,
		filter:              subscriptionsapi.NewAllFilter(subscriptionsapi.MaterializeFiltersList(logger.Desugar(), []eventingv1.SubscriptionsAPIFilter{})...),
		allowedNamespaces:   allowedNs,
		filterByNamespace:   true,
	}

	// Test allowed namespaces - each should add one event
	expectedCount := 0
	for _, ns := range []string{"prod-1", "prod-2", "prod-3"} {
		pod := simplePod("test-pod", ns)
		err := delegate.Add(pod)
		if err != nil {
			t.Fatalf("unexpected error for namespace %s: %v", ns, err)
		}
		expectedCount++
		if got := len(ce.Sent()); got != expectedCount {
			t.Errorf("Expected %d events after namespace %s, got: %d", expectedCount, ns, got)
		}
	}

	// Test denied namespace - count should stay the same
	deniedPod := simplePod("test-pod", "dev-1")
	err := delegate.Add(deniedPod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(ce.Sent()); got != expectedCount {
		t.Errorf("Expected %d events (denied namespace shouldn't add), got: %d", expectedCount, got)
	}
}
