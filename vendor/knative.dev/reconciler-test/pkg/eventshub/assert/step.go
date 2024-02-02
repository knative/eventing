package assert

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"

	cetest "github.com/cloudevents/sdk-go/v2/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"

	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
)

type MatchAssertionBuilder struct {
	storeName string
	matchers  []eventshub.EventInfoMatcherCtx
}

// OnStore creates an assertion builder starting from the name of the store
func OnStore(name string) MatchAssertionBuilder {
	return MatchAssertionBuilder{
		storeName: name,
		matchers:  nil,
	}
}

// Match adds the provided matchers in this builder
func (m MatchAssertionBuilder) Match(matchers ...eventshub.EventInfoMatcher) MatchAssertionBuilder {
	for _, matcher := range matchers {
		m.matchers = append(m.matchers, matcher.WithContext())
	}
	return m
}

// MatchWithContext adds the provided matchers in this builder
func (m MatchAssertionBuilder) MatchWithContext(matchers ...eventshub.EventInfoMatcherCtx) MatchAssertionBuilder {
	m.matchers = append(m.matchers, matchers...)
	return m
}

// MatchPeerCertificates adds the provided matchers in this builder
func (m MatchAssertionBuilder) MatchPeerCertificatesReceived(matchers ...eventshub.EventInfoMatcherCtx) MatchAssertionBuilder {
	m.matchers = append(m.matchers, MatchKind(eventshub.PeerCertificatesReceived).WithContext())
	m.matchers = append(m.matchers, matchers...)
	return m
}

// MatchReceivedEvent is a shortcut for Match(MatchKind(eventshub.EventReceived), MatchEvent(matchers...))
func (m MatchAssertionBuilder) MatchReceivedEvent(matchers ...cetest.EventMatcher) MatchAssertionBuilder {
	m.matchers = append(m.matchers, MatchKind(eventshub.EventReceived).WithContext())
	m.matchers = append(m.matchers, MatchEvent(matchers...).WithContext())
	return m
}

// MatchRejectedEvent is a shortcut for Match(MatchKind(eventshub.EventRejected), MatchEvent(matchers...))
func (m MatchAssertionBuilder) MatchRejectedEvent(matchers ...cetest.EventMatcher) MatchAssertionBuilder {
	m.matchers = append(m.matchers, MatchKind(eventshub.EventRejected).WithContext())
	m.matchers = append(m.matchers, MatchEvent(matchers...).WithContext())
	return m
}

// MatchSentEvent is a shortcut for Match(MatchKind(eventshub.EventSent), MatchEvent(matchers...))
func (m MatchAssertionBuilder) MatchSentEvent(matchers ...cetest.EventMatcher) MatchAssertionBuilder {
	m.matchers = append(m.matchers, MatchKind(eventshub.EventSent).WithContext())
	m.matchers = append(m.matchers, MatchEvent(matchers...).WithContext())
	return m
}

// MatchResponseEvent is a shortcut for Match(MatchKind(eventshub.EventResponse), MatchEvent(matchers...))
func (m MatchAssertionBuilder) MatchResponseEvent(matchers ...cetest.EventMatcher) MatchAssertionBuilder {
	m.matchers = append(m.matchers, MatchKind(eventshub.EventResponse).WithContext())
	m.matchers = append(m.matchers, MatchEvent(matchers...).WithContext())
	return m
}

// MatchEvent is a shortcut for Match(MatchEvent(), OneOf(MatchKind(eventshub.EventReceived), MatchKind(eventshub.EventSent)))
func (m MatchAssertionBuilder) MatchEvent(matchers ...cetest.EventMatcher) MatchAssertionBuilder {
	m.matchers = append(m.matchers, OneOf(
		MatchKind(eventshub.EventReceived),
		MatchKind(eventshub.EventSent),
	).WithContext())
	m.matchers = append(m.matchers, MatchEvent(matchers...).WithContext())
	return m
}

// AtLeast builds the assertion feature.StepFn
// OnStore(store).Match(matchers).AtLeast(min) is equivalent to StoreFromContext(ctx, store).AssertAtLeast(min, matchers)
func (m MatchAssertionBuilder) AtLeast(min int) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		eventshub.StoreFromContext(ctx, m.storeName).AssertAtLeast(ctx, t, min, toFixedContextMatchers(ctx, m.matchers)...)
	}
}

// InRange builds the assertion feature.StepFn
// OnStore(store).Match(matchers).InRange(min, max) is equivalent to StoreFromContext(ctx, store).AssertInRange(min, max, matchers)
func (m MatchAssertionBuilder) InRange(min int, max int) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		eventshub.StoreFromContext(ctx, m.storeName).AssertInRange(ctx, t, min, max, toFixedContextMatchers(ctx, m.matchers)...)
	}
}

// Exact builds the assertion feature.StepFn
// OnStore(store).Match(matchers).Exact(n) is equivalent to StoreFromContext(ctx, store).AssertExact(n, matchers)
func (m MatchAssertionBuilder) Exact(n int) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		eventshub.StoreFromContext(ctx, m.storeName).AssertExact(ctx, t, n, toFixedContextMatchers(ctx, m.matchers)...)
	}
}

// Not builds the assertion feature.StepFn
// OnStore(store).Match(matchers).Not() is equivalent to StoreFromContext(ctx, store).AssertNot(matchers)
func (m MatchAssertionBuilder) Not() feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		eventshub.StoreFromContext(ctx, m.storeName).AssertNot(t, toFixedContextMatchers(ctx, m.matchers)...)
	}
}

func toFixedContextMatchers(ctx context.Context, matchers []eventshub.EventInfoMatcherCtx) []eventshub.EventInfoMatcher {
	out := make([]eventshub.EventInfoMatcher, 0, len(matchers))
	for _, matcher := range matchers {
		out = append(out, matcher.WithContext(ctx))
	}
	return out
}

func MatchPeerCertificatesFromSecret(namespace, name string, key string) eventshub.EventInfoMatcherCtx {
	return func(ctx context.Context, info eventshub.EventInfo) error {
		secret, err := kubeclient.Get(ctx).CoreV1().
			Secrets(namespace).
			Get(ctx, name, metav1.GetOptions{})

		if err != nil {
			return fmt.Errorf("failed to get secret: %w", err)
		}

		value, ok := secret.Data[key]
		if !ok {
			return fmt.Errorf("failed to get value from secret %s/%s for key %s", secret.Namespace, secret.Name, key)
		}

		if info.Connection == nil || info.Connection.TLS == nil {
			return fmt.Errorf("failed to match peer certificates, connection is not TLS")
		}

		// secret value can, in general, be a certificate chain (a sequence of PEM-encoded certificate blocks)
		valueBlock, valueRest := pem.Decode(value)
		if valueBlock == nil {
			// error if there's not even a single certificate in the value
			return fmt.Errorf("failed to decode secret certificate:\n%s", string(value))
		}
		// for each certificate in the chain, check if it's present in info.Connection.TLS.PemPeerCertificates
		for valueBlock != nil {
			found := false
			for _, cert := range info.Connection.TLS.PemPeerCertificates {
				certBlock, _ := pem.Decode([]byte(cert))
				if certBlock == nil {
					return fmt.Errorf("failed to decode peer certificate:\n%s", cert)
				}

				if certBlock.Type == valueBlock.Type && string(certBlock.Bytes) == string(valueBlock.Bytes) {
					found = true
					break
				}
			}

			if !found {
				pemBytes, _ := json.MarshalIndent(info.Connection.TLS.PemPeerCertificates, "", "  ")
				return fmt.Errorf("failed to find peer certificate with value\n%s\nin:\n%s", string(value), string(pemBytes))
			}

			valueBlock, valueRest = pem.Decode(valueRest)
		}

		// any non-whitespace suffix not parsed as a PEM is suspicious, so we treat it as an error:
		if "" != string(bytes.TrimSpace(valueRest)) {
			return fmt.Errorf("failed to decode secret certificate starting with\n%s\nin:\n%s", string(valueRest), string(value))
		}

		return nil
	}
}
