/*
Copyright 2020 The Knative Authors

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

package inmemorychannel

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"sync"
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/transformer"
	protocolhttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/cloudevents/sdk-go/v2/test"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"knative.dev/pkg/apis"

	"knative.dev/pkg/tracing"
	tracingconfig "knative.dev/pkg/tracing/config"

	"knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/channel/fanout"
	"knative.dev/eventing/pkg/channel/multichannelfanout"
	"knative.dev/eventing/pkg/kncloudevents"

	logtesting "knative.dev/pkg/logging/testing"
)

func TestNewMessageDispatcher(t *testing.T) {
	logger := logtesting.TestLogger(t).Desugar()
	reporter := channel.NewStatsReporter("testcontainer", "testpod")
	sh := multichannelfanout.NewMessageHandler(context.TODO(), logger, channel.NewMessageDispatcher(logger), reporter)

	args := &InMemoryMessageDispatcherArgs{
		Port:         8080,
		ReadTimeout:  1 * time.Minute,
		WriteTimeout: 1 * time.Minute,
		Handler:      sh,
		Logger:       logger,
	}

	d := NewMessageDispatcher(args)

	if d == nil {
		t.Fatalf("Failed to create with NewDispatcher")
	}
}

// This test emulates a real dispatcher usage
func TestDispatcher_close(t *testing.T) {
	logger := logtesting.TestLogger(t).Desugar()
	reporter := channel.NewStatsReporter("testcontainer", "testpod")
	sh := multichannelfanout.NewMessageHandler(context.TODO(), logger, channel.NewMessageDispatcher(logger), reporter)

	port, err := freePort()
	if err != nil {
		t.Fatal(err)
	}

	dispatcherArgs := &InMemoryMessageDispatcherArgs{
		Port:         port,
		ReadTimeout:  1 * time.Minute,
		WriteTimeout: 1 * time.Minute,
		Handler:      sh,
		Logger:       logger,
	}

	dispatcher := NewMessageDispatcher(dispatcherArgs)
	kncloudevents.WithDrainQuietPeriod(time.Nanosecond)(dispatcher.httpBindingsReceiver)

	serverCtx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- dispatcher.Start(serverCtx)
	}()

	cancel()
	err = <-errCh
	if err != nil {
		t.Fatal(err)
	}
}

// This test emulates a real dispatcher usage
func TestDispatcher_dispatch(t *testing.T) {
	logger, err := zap.NewDevelopment(zap.AddStacktrace(zap.WarnLevel))
	reporter := channel.NewStatsReporter("testcontainer", "testpod")
	if err != nil {
		t.Fatal(err)
	}

	// tracing publishing is configured to let the code pass in all "critical" branches
	tracer, _ := tracing.SetupPublishingWithStaticConfig(logger.Sugar(), "localhost", &tracingconfig.Config{
		Backend:        tracingconfig.Zipkin,
		Debug:          true,
		SampleRate:     1.0,
		ZipkinEndpoint: "http://zipkin.zipkin.svc.cluster.local:9411/api/v2/spans",
	})
	defer tracer.Shutdown(context.Background())

	port, err := freePort()
	if err != nil {
		t.Fatal(err)
	}

	// We need a channelaproxy and channelbproxy for handling correctly the Host header
	channelAProxy := httptest.NewServer(createReverseProxy(t, "channela.svc", port))
	defer channelAProxy.Close()
	channelBProxy := httptest.NewServer(createReverseProxy(t, "channelb.svc", port))
	defer channelBProxy.Close()

	// Start a bunch of test servers to simulate the various services
	transformationsCh := make(chan error, 1)
	transformationsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		message := protocolhttp.NewMessageFromHttpRequest(r)
		defer message.Finish(nil)

		if message.ReadEncoding() == binding.EncodingUnknown {
			err := errors.New("Message has encoding unknown")
			logger.Fatal("Error in transformation handler", zap.Error(err))
			transformationsCh <- err
		}

		err := protocolhttp.WriteResponseWriter(context.Background(), message, 200, w, transformer.AddExtension("transformed", "true"))
		if err != nil {
			logger.Fatal("Error in transformation handler", zap.Error(err))
			transformationsCh <- err
			w.WriteHeader(http.StatusInternalServerError)
		}
		transformationsCh <- nil
	}))
	defer transformationsServer.Close()

	receiverWg := sync.WaitGroup{}
	receiverWg.Add(1)
	receiverCh := make(chan error, 1)
	receiverServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receiverWg.Done()
		transformed := r.Header.Get("ce-transformed")
		if transformed != "true" {
			err := fmt.Errorf("expecting ce-transformed: true, found %s", transformed)
			logger.Fatal("Error in receiver", zap.Error(err))
			receiverCh <- err
			w.WriteHeader(http.StatusInternalServerError)
		}
		receiverCh <- nil
	}))
	defer receiverServer.Close()

	transformationsFailureWg := sync.WaitGroup{}
	transformationsFailureWg.Add(1)
	transformationsFailureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer transformationsFailureWg.Done()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer transformationsFailureServer.Close()

	deadLetterWg := sync.WaitGroup{}
	deadLetterWg.Add(1)
	deadLetterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer deadLetterWg.Done()
		transformed := r.Header.Get("ce-transformed")
		if transformed != "" {
			w.WriteHeader(http.StatusInternalServerError)
			t.Fatal("Not expecting ce-transformed, found", transformed)
		}
	}))
	defer deadLetterServer.Close()

	logger.Debug("Test servers",
		zap.String("transformations server", transformationsServer.URL),
		zap.String("transformations failure server", transformationsFailureServer.URL),
		zap.String("receiver server", receiverServer.URL),
		zap.String("dead letter server", deadLetterServer.URL),
	)

	// The message flow is:
	// send -> channela -> sub aaaa -> transformationsServer -> channelb -> sub bbbb -> receiver
	// send -> channela -> sub cccc -> transformationsFailureServer
	config := multichannelfanout.Config{
		ChannelConfigs: []multichannelfanout.ChannelConfig{
			{
				Namespace: "default",
				Name:      "channela",
				HostName:  "channela.svc",
				FanoutConfig: fanout.Config{
					AsyncHandler: false,
					Subscriptions: []fanout.Subscription{{
						Subscriber: mustParseUrl(t, transformationsServer.URL).URL(),
						Reply:      mustParseUrl(t, channelBProxy.URL).URL(),
					}, {
						Subscriber: mustParseUrl(t, transformationsFailureServer.URL).URL(),
						Reply:      mustParseUrl(t, channelBProxy.URL).URL(),
						DeadLetter: mustParseUrl(t, deadLetterServer.URL).URL(),
					}},
				},
			},
			{
				Namespace: "default",
				Name:      "channelb",
				HostName:  "channelb.svc",
				FanoutConfig: fanout.Config{
					AsyncHandler: false,
					Subscriptions: []fanout.Subscription{{
						Subscriber: mustParseUrl(t, receiverServer.URL).URL(),
					}},
				},
			},
		},
	}

	sh, err := multichannelfanout.NewMessageHandlerWithConfig(context.TODO(), logger, channel.NewMessageDispatcher(logger), config, reporter)
	if err != nil {
		t.Fatal(err)
	}

	logger.Info("Starting dispatcher", zap.Int("port", port))

	dispatcherArgs := &InMemoryMessageDispatcherArgs{
		Port:         port,
		ReadTimeout:  1 * time.Minute,
		WriteTimeout: 1 * time.Minute,
		Handler:      sh,
		Logger:       logger,
	}

	dispatcher := NewMessageDispatcher(dispatcherArgs)

	serverCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the dispatcher
	go func() {
		if err := dispatcher.Start(serverCtx); err != nil {
			t.Error(err)
		}
	}()
	dispatcher.WaitReady()

	// Ok now everything should be ready to send the event
	httpsender, err := kncloudevents.NewHTTPMessageSenderWithTarget(channelAProxy.URL)
	if err != nil {
		t.Fatal(err)
	}

	req, err := httpsender.NewCloudEventRequest(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	event := test.FullEvent()
	_ = protocolhttp.WriteRequest(context.Background(), binding.ToMessage(&event), req)

	res, err := httpsender.Send(req)
	if err != nil {
		t.Fatal(err)
	}

	if res.StatusCode != http.StatusAccepted {
		t.Fatal("Expected 202, Have", res.StatusCode)
	}

	transformationsFailureWg.Wait()
	deadLetterWg.Wait()
	err = <-transformationsCh
	if err != nil {
		t.Fatal(err)
	}
	receiverWg.Wait()
	err = <-receiverCh
	if err != nil {
		t.Fatal(err)
	}
}

func createReverseProxy(t *testing.T, host string, port int) *httputil.ReverseProxy {
	director := func(req *http.Request) {
		target := mustParseUrl(t, fmt.Sprint("http://localhost:", port))
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = target.Path
		req.Host = host
	}
	return &httputil.ReverseProxy{Director: director}
}

func mustParseUrl(t testing.TB, str string) *apis.URL {
	url, err := apis.ParseURL(str)
	if err != nil {
		t.Fatal(err)
	}
	return url
}

func freePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
