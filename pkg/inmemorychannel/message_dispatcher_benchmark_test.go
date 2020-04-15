package inmemorychannel

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/test"
	"github.com/cloudevents/sdk-go/v2/binding/transformer"
	protocolhttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.uber.org/zap"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/channel/fanout"
	"knative.dev/eventing/pkg/channel/multichannelfanout"
	"knative.dev/eventing/pkg/channel/swappable"
	"knative.dev/eventing/pkg/kncloudevents"
)

type roundTripFunc func(req *http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func mockedHttpClient(fn roundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

// This test emulates a real dispatcher usage
func BenchmarkDispatcher_dispatch(b *testing.B) {
	logger, err := zap.NewDevelopment(zap.AddStacktrace(zap.WarnLevel))
	if err != nil {
		b.Fatal(err)
	}

	sh, err := swappable.NewEmptyMessageHandler(context.TODO(), logger)
	if err != nil {
		b.Fatal(err)
	}

	port, err := freePort()
	if err != nil {
		b.Fatal(err)
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
			b.Fatal(err)
		}
	}()

	// We need a channelaproxy and channelbproxy for handling correctly the Host header
	channelAProxy := httptest.NewServer(createReverseProxy(b, "channela.svc", port))
	defer channelAProxy.Close()
	channelBProxy := httptest.NewServer(createReverseProxy(b, "channelb.svc", port))
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
			w.WriteHeader(500)
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
			w.WriteHeader(500)
		}
		receiverCh <- nil
	}))
	defer receiverServer.Close()

	transformationsFailureWg := sync.WaitGroup{}
	transformationsFailureWg.Add(1)
	transformationsFailureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer transformationsFailureWg.Done()
		w.WriteHeader(500)
	}))
	defer transformationsFailureServer.Close()

	deadLetterWg := sync.WaitGroup{}
	deadLetterWg.Add(1)
	deadLetterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer deadLetterWg.Done()
		transformed := r.Header.Get("ce-transformed")
		if transformed != "" {
			w.WriteHeader(500)
			b.Fatalf("Not expecting ce-transformed, found %s", transformed)
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
					AsyncHandler:     false,
					DispatcherConfig: channel.EventDispatcherConfig{},
					Subscriptions: []eventingduck.SubscriberSpec{{
						UID:           "aaaa",
						Generation:    1,
						SubscriberURI: mustParseUrl(b, transformationsServer.URL),
						ReplyURI:      mustParseUrl(b, channelBProxy.URL),
					}, {
						UID:           "cccc",
						Generation:    1,
						SubscriberURI: mustParseUrl(b, transformationsFailureServer.URL),
						ReplyURI:      mustParseUrl(b, channelBProxy.URL),
						Delivery: &eventingduck.DeliverySpec{
							DeadLetterSink: &duckv1.Destination{URI: mustParseUrl(b, deadLetterServer.URL)},
						},
					}},
				},
			},
			{
				Namespace: "default",
				Name:      "channelb",
				HostName:  "channelb.svc",
				FanoutConfig: fanout.Config{
					AsyncHandler:     false,
					DispatcherConfig: channel.EventDispatcherConfig{},
					Subscriptions: []eventingduck.SubscriberSpec{{
						UID:           "bbbb",
						Generation:    1,
						SubscriberURI: mustParseUrl(b, receiverServer.URL),
					}},
				},
			},
		},
	}

	err = sh.UpdateConfig(context.TODO(), &config)
	if err != nil {
		b.Fatal(err)
	}

	// Ok now everything should be ready to send the event
	httpsender, err := kncloudevents.NewHttpMessageSender(nil, channelAProxy.URL)
	httpsender.Client = http.Client{Transport: http.RoundTripper()}
	if err != nil {
		b.Fatal(err)
	}

	req, err := httpsender.NewCloudEventRequest(context.Background())
	if err != nil {
		b.Fatal(err)
	}

	event := test.FullEvent()
	_ = protocolhttp.WriteRequest(context.Background(), binding.ToMessage(&event), req)

	res, err := httpsender.Send(req)
	if err != nil {
		b.Fatal(err)
	}

	if res.StatusCode != 202 {
		b.Fatalf("Expected 202, Have %d", res.StatusCode)
	}

	transformationsFailureWg.Wait()
	deadLetterWg.Wait()
	err = <-transformationsCh
	if err != nil {
		b.Fatal(err)
	}
	receiverWg.Wait()
	err = <-receiverCh
	if err != nil {
		b.Fatal(err)
	}
}
