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

package sender_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/openzipkin/zipkin-go/model"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"go.opencensus.io/trace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/eventing/test/upgrade/prober/wathola/client"
	"knative.dev/eventing/test/upgrade/prober/wathola/receiver"
	"knative.dev/eventing/test/upgrade/prober/wathola/sender"
	"knative.dev/pkg/tracing"
	"knative.dev/pkg/tracing/config"
	tracetesting "knative.dev/pkg/tracing/testing"
)

func TestHTTPEventSender(t *testing.T) {
	sender.ResetEventSenders()
	ce := sender.NewCloudEvent(nil, "cetype")
	port := freeport.GetPort()
	canceling := make(chan context.CancelFunc, 1)
	events := make([]cloudevents.Event, 0, 1)
	go client.Receive(port, canceling, func(ctx context.Context, e cloudevents.Event) {
		events = append(events, e)
	})
	cancel := <-canceling
	waitForPort(t, port)
	err := sender.SendEvent(context.Background(), ce, fmt.Sprintf("http://localhost:%d", port))
	cancel()
	assert.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, ce, events[0])
}

func TestTracePropagation(t *testing.T) {
	sender.ResetEventSenders()
	reporter, co := tracetesting.FakeZipkinExporter()
	oct := tracing.NewOpenCensusTracer(co)
	t.Cleanup(func() {
		reporter.Close()
		oct.Finish()
	})
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.ConfigName,
		},
		Data: map[string]string{
			"backend":         "zipkin",
			"zipkin-endpoint": "foo.bar",
			"debug":           "true",
		},
	}
	cfg, err := config.NewTracingConfigFromConfigMap(cm)
	if err != nil {
		t.Fatal("Failed to parse tracing config", err)
	}
	if err := oct.ApplyConfig(cfg); err != nil {
		t.Fatal("Failed to apply tracer config:", err)
	}
	ce := sender.NewCloudEvent(nil, "cetype")
	port := freeport.GetPort()
	canceling := make(chan context.CancelFunc, 1)
	go client.Receive(port, canceling, func(ctx context.Context, e cloudevents.Event) {
		_, span := trace.StartSpan(ctx, receiver.Name)
		span.End()
	})

	cancel := <-canceling
	waitForPort(t, port)
	ctx, span := sender.PopulateSpanWithEvent(context.Background(), ce, sender.Name)
	err = sender.SendEvent(ctx, ce, fmt.Sprintf("http://localhost:%d", port))
	cancel()
	span.End()
	spans := reporter.Flush()
	assert.NoError(t, err)
	assert.Len(t, spans, 4)
	assert.Equal(t, receiver.Name, spans[0].Name)
	// Generated by tracing Middleware in Receiver.
	assert.Equal(t, model.Server, spans[1].Kind)
	// Generated by tracing RoundTripper in Sender.
	assert.Equal(t, model.Client, spans[2].Kind)
	assert.Equal(t, sender.Name, spans[3].Name)
	// Verify the trace is uninterrupted.
	for i := 0; i != len(spans)-1; i++ {
		assert.Equal(t, *spans[i].ParentID, spans[i+1].ID)
	}
}

func TestRegisterEventSender(t *testing.T) {
	sender.RegisterEventSender(testEventSender{})
	defer sender.ResetEventSenders()
	c := testConfig{
		topic:   "sample",
		servers: "example.org:9290",
		valid:   true,
	}
	ce := sender.NewCloudEvent(nil, "cetype")
	err := sender.SendEvent(context.Background(), ce, c)

	assert.NoError(t, err)
}

func TestUnsupportedEventSender(t *testing.T) {
	sender.RegisterEventSender(testEventSender{})
	defer sender.ResetEventSenders()
	ce := sender.NewCloudEvent(nil, "cetype")
	err := sender.SendEvent(context.Background(), ce, "https://example.org/")

	assert.Error(t, err)
	assert.True(t, errors.Is(err, sender.ErrEndpointTypeNotSupported))
}

func waitForPort(t *testing.T, port int) {
	if err := wait.PollImmediate(time.Millisecond, 10*time.Second, func() (bool, error) {
		conn, conErr := net.Dial("tcp", fmt.Sprintf(":%d", port))
		defer func() {
			if conn != nil {
				assert.NoError(t, conn.Close())
			}
		}()
		if conErr != nil {
			return false, nil
		}
		return true, nil
	}); err != nil {
		t.Fatalf("%v: port %d to be open", err, port)
	}
}

type testEventSender struct{}

type testConfig struct {
	topic   string
	servers string
	valid   bool
}

func (t testEventSender) Supports(endpoint interface{}) bool {
	switch endpoint.(type) {
	case testConfig:
		return true
	default:
		return false
	}
}

func (t testEventSender) SendEvent(ce cloudevents.Event, endpoint interface{}) error {
	return t.SendEventWithContext(context.Background(), ce, endpoint)
}

func (t testEventSender) SendEventWithContext(ctx context.Context, ce cloudevents.Event, endpoint interface{}) error {
	cfg := endpoint.(testConfig)
	if cfg.valid {
		return nil
	}
	return fmt.Errorf("can't send %#v to %s server at %s topic",
		ce, cfg.servers, cfg.topic)
}
