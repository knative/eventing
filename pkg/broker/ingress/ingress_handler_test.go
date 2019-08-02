package ingress

import (
	"context"
	nethttp "net/http"
	"net/url"
	"testing"

	cloudevents2 "github.com/cloudevents/sdk-go/pkg/cloudevents"

	cloudevents "github.com/cloudevents/sdk-go"
	"go.uber.org/zap"

	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
)

var (
	logger     = zap.NewNop()
	channelURI = &url.URL{
		Scheme: "http",
		Host:   "testChannel",
		Path:   "/",
	}
	brokerName = "testBroker"
	validURI   = "/"
	event      = cloudevents.NewEvent()
	resp       = new(cloudevents.EventResponse)
)

type fakeClient struct{ sent bool }

func (f *fakeClient) Send(ctx context.Context, event cloudevents2.Event) (*cloudevents2.Event, error) {
	f.sent = true
	return &event, nil
}

func (f *fakeClient) StartReceiver(ctx context.Context, fn interface{}) error {
	panic("not implemented")
}

func TestIngressHandler_ServeHTTP_FAIL(t *testing.T) {
	testCases := map[string]struct {
		httpmethod     string
		URI            string
		expectedStatus int
	}{
		"method not allowed": {
			httpmethod:     nethttp.MethodGet,
			URI:            validURI,
			expectedStatus: nethttp.StatusMethodNotAllowed,
		},
		"invalid url": {
			httpmethod:     nethttp.MethodPost,
			URI:            "invalidURI",
			expectedStatus: nethttp.StatusNotFound,
		},
	}

	for n, tc := range testCases {
		client, _ := cloudevents.NewDefaultClient()
		t.Run(n, func(t *testing.T) {
			handler := Handler{
				Logger:     logger,
				CeClient:   client,
				ChannelURI: channelURI,
				BrokerName: brokerName,
			}
			tctx := http.TransportContext{Method: tc.httpmethod, URI: tc.URI}
			ctx := http.WithTransportContext(context.Background(), tctx)
			_ = handler.serveHTTP(ctx, event, resp)
			if resp.Status != tc.expectedStatus {
				t.Errorf("Unexpected status code. Expected %v, Actual %v", tc.expectedStatus, resp.Status)
			}

		})
	}

}

func TestIngressHandler_ServeHTTP_Succeed(t *testing.T) {

	client := &fakeClient{false}
	handler := Handler{
		Logger:     logger,
		CeClient:   client,
		ChannelURI: channelURI,
		BrokerName: brokerName,
	}

	tctx := http.TransportContext{Method: nethttp.MethodPost, URI: "/"}
	ctx := http.WithTransportContext(context.Background(), tctx)
	_ = handler.serveHTTP(ctx, event, resp)
	if !client.sent {
		t.Errorf("client should invoke send function")
	}
}
