package ingress

import (
	"context"
	nethttp "net/http"
	"net/url"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"go.uber.org/zap"
)

const (
	namespace       = "testNamespace"
	brokerName      = "testBroker"
	validURI        = "/"
	urlHost         = "testHost"
	urlPath         = "/"
	urlScheme       = "http"
	validHTTPMethod = nethttp.MethodPost
)

type mockReporter struct {
	eventCountReported        bool
	eventDispatchTimeReported bool
}

func (r *mockReporter) ReportEventCount(args *ReportArgs, responseCode int) error {
	r.eventCountReported = true
	return nil
}

func (r *mockReporter) ReportEventDispatchTime(args *ReportArgs, responseCode int, d time.Duration) error {
	r.eventDispatchTimeReported = true
	return nil
}

type fakeClient struct{ sent bool }

func (f *fakeClient) Send(ctx context.Context, event cloudevents.Event) (context.Context, *cloudevents.Event, error) {
	f.sent = true
	return ctx, &event, nil
}

func (f *fakeClient) StartReceiver(ctx context.Context, fn interface{}) error {
	panic("not implemented")
}

func TestIngressHandler_ServeHTTP_FAIL(t *testing.T) {
	testCases := map[string]struct {
		httpmethod                string
		URI                       string
		expectedStatus            int
		expectedEventCount        bool
		expectedEventDispatchTime bool
	}{
		"method not allowed": {
			httpmethod:     nethttp.MethodGet,
			URI:            validURI,
			expectedStatus: nethttp.StatusMethodNotAllowed,
		},
		"invalid url": {
			httpmethod:     validHTTPMethod,
			URI:            "invalidURI",
			expectedStatus: nethttp.StatusNotFound,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			client, _ := cloudevents.NewDefaultClient()
			reporter := &mockReporter{}
			handler := Handler{
				Logger:   zap.NewNop(),
				CeClient: client,
				ChannelURI: &url.URL{
					Scheme: urlScheme,
					Host:   urlHost,
					Path:   urlPath,
				},
				BrokerName: brokerName,
				Namespace:  namespace,
				Reporter:   reporter,
			}
			event := cloudevents.NewEvent(cloudevents.VersionV1)
			resp := new(cloudevents.EventResponse)
			tctx := http.TransportContext{Header: nethttp.Header{}, Method: tc.httpmethod, URI: tc.URI}
			ctx := http.WithTransportContext(context.Background(), tctx)
			_ = handler.serveHTTP(ctx, event, resp)
			if resp.Status != tc.expectedStatus {
				t.Errorf("Unexpected status code. Expected %v, Actual %v", tc.expectedStatus, resp.Status)
			}
			if reporter.eventCountReported != tc.expectedEventCount {
				t.Errorf("Unexpected event count reported. Expected %v, Actual %v", tc.expectedEventCount, reporter.eventCountReported)
			}
			if reporter.eventDispatchTimeReported != tc.expectedEventDispatchTime {
				t.Errorf("Unexpected event dispatch time reported. Expected %v, Actual %v", tc.expectedEventDispatchTime, reporter.eventDispatchTimeReported)
			}
		})
	}
}

func TestIngressHandler_ServeHTTP_Succeed(t *testing.T) {
	client := &fakeClient{}
	reporter := &mockReporter{}
	handler := Handler{
		Logger:   zap.NewNop(),
		CeClient: client,
		ChannelURI: &url.URL{
			Scheme: urlScheme,
			Host:   urlHost,
			Path:   urlPath,
		},
		BrokerName: brokerName,
		Namespace:  namespace,
		Reporter:   reporter,
	}
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	resp := new(cloudevents.EventResponse)
	tctx := http.TransportContext{Header: nethttp.Header{}, Method: validHTTPMethod, URI: validURI}
	ctx := http.WithTransportContext(context.Background(), tctx)
	_ = handler.serveHTTP(ctx, event, resp)
	if !client.sent {
		t.Errorf("client should invoke send function")
	}
	if !reporter.eventCountReported {
		t.Errorf("event count should have been reported")
	}
	if !reporter.eventDispatchTimeReported {
		t.Errorf("event dispatch time should have been reported")
	}
}
