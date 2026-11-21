/*
Copyright 2026 The Knative Authors

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

package filter

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"

	"github.com/cloudevents/sdk-go/v2/event"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	configmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/fake"
	filteredconfigmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/filtered/fake"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/observability/metrics/metricstest"
	reconcilertesting "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/system"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/eventing/pkg/auth"
	brokerinformerfake "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/broker/fake"
	triggerinformerfake "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/trigger/fake"
	eventpolicyinformerfake "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventpolicy/fake"
	subscriptioninformerfake "knative.dev/eventing/pkg/client/injection/informers/messaging/v1/subscription/fake"
	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/observability"

	_ "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret/fake"
)

const globalProvidersEnv = "SERVER_MANAGER_TEST_GLOBAL_PROVIDERS"

func TestServerManagerInstrumentsEachRequestOnce(t *testing.T) {
	if os.Getenv(globalProvidersEnv) == "" {
		cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$")
		cmd.Env = append(os.Environ(), globalProvidersEnv+"=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return
	}

	const requests = 3
	const traceParent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

	ctx, _ := reconcilertesting.SetupFakeContext(t, SetUpInformerSelector)
	logger := zap.NewNop()

	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))

	otel.SetMeterProvider(mp)
	otel.SetTracerProvider(tp)

	subscriber := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer subscriber.Close()

	subscriberURI, err := apis.ParseURL(subscriber.URL)
	if err != nil {
		t.Fatal(err)
	}
	trigger := makeTrigger()
	trigger.Spec.Broker = "default"
	trigger.Status.SubscriberURI = subscriberURI
	triggerinformerfake.Get(ctx).Informer().GetStore().Add(trigger)
	brokerinformerfake.Get(ctx).Informer().GetStore().Add(&eventingv1.Broker{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: testNS},
	})

	trustBundleConfigMapLister := filteredconfigmapinformer.Get(ctx, eventingtls.TrustBundleLabelSelector).Lister().ConfigMaps(system.Namespace())
	cmw := configmap.NewStaticWatcher(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      feature.FlagsConfigName,
			Namespace: system.Namespace(),
		},
		Data: map[string]string{
			feature.TransportEncryption: string(feature.Disabled),
		},
	})

	h, err := NewHandler(
		logger,
		auth.NewVerifier(ctx, eventpolicyinformerfake.Get(ctx).Lister(), trustBundleConfigMapLister, cmw),
		auth.NewOIDCTokenProvider(ctx),
		triggerinformerfake.Get(ctx),
		brokerinformerfake.Get(ctx),
		subscriptioninformerfake.Get(ctx),
		configmapinformer.Get(ctx).Lister().ConfigMaps("ns"),
		func(ctx context.Context) context.Context {
			return ctx
		},
		mp,
		tp,
	)
	if err != nil {
		t.Fatal("Unable to create handler:", err)
	}

	httpReceiver := kncloudevents.NewHTTPEventReceiver(0)
	sm, err := newServerManager(ctx, cmw, httpReceiver, kncloudevents.NewHTTPEventReceiver(0), mp, tp, h)
	if err != nil {
		t.Fatal("Unable to create server manager:", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	errCh := make(chan error, 1)
	go func() {
		errCh <- sm.StartServers(ctx)
	}()

	addr := fmt.Sprintf("127.0.0.1:%d", httpReceiver.GetPort())

	body, err := makeEvent().MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for range requests {
		req, err := http.NewRequest(http.MethodPost, "http://"+addr+validPath, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set(cehttp.ContentType, event.ApplicationCloudEventsJSON)
		req.Header.Set("traceparent", traceParent)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("expected status code %d, got %d", http.StatusAccepted, resp.StatusCode)
		}
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatal("StartServers returned an error:", err)
	}

	if got := requestCount(t, reader); got != requests {
		t.Errorf("expected http.server.request.duration to count %d requests, got %d", requests, got)
	}
	metricstest.AssertMetrics(t, reader, metricstest.HasAttributes(otelhttp.ScopeName, "http.server.request.duration",
		observability.BrokerName.With("default"),
		observability.BrokerNamespace.With(testNS),
		observability.CloudEventType.With(eventType),
		observability.RequestScheme.With("http"),
	))

	serverSpans := 0
	for _, span := range exporter.GetSpans() {
		if span.SpanKind != trace.SpanKindServer {
			continue
		}
		serverSpans++
		if want := "POST " + validPath; span.Name != want {
			t.Errorf("expected server span name %q, got %q", want, span.Name)
		}
		if got := span.Parent.SpanID().String(); !span.Parent.IsRemote() || got != "00f067aa0ba902b7" {
			t.Errorf("expected server span to be a child of the caller's span, got parent %s (remote: %t)", got, span.Parent.IsRemote())
		}
	}
	if serverSpans != requests {
		t.Errorf("expected %d server spans, got %d", requests, serverSpans)
	}
}

func requestCount(t *testing.T, reader sdkmetric.Reader) (count uint64) {
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal("Unable to collect metrics:", err)
	}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if data, ok := m.Data.(metricdata.Histogram[float64]); ok && m.Name == "http.server.request.duration" {
				for _, dp := range data.DataPoints {
					count += dp.Count
				}
			}
		}
	}
	return count
}
