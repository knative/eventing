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

package receiver

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	cloudeventsbindings "github.com/cloudevents/sdk-go/v2/binding"
	cloudeventshttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"

	"knative.dev/reconciler-test/pkg/eventshub"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/reconciler-test/pkg/eventshub/dropevents"
)

// Receiver is the entry point for sinking events into the event log.
type Receiver struct {
	// Name is the name of this Receiver, used to filter if multiple observers.
	Name string
	// EventLogs is the list of EventLogger implementors to vent observed events.
	EventLogs *eventshub.EventLogs

	ctx                 context.Context
	seq                 uint64
	dropSeq             uint64
	replyFunc           func(context.Context, http.ResponseWriter, eventshub.EventInfo)
	counter             *dropevents.CounterHandler
	responseWaitTime    time.Duration
	skipResponseCode    int
	skipResponseHeaders map[string]string
	skipResponseBody    string
	EnforceTLS          bool
	oidcAudience        string
	expectedFormat      *string

	kubeclient kubernetes.Interface
}

type envConfig struct {
	// ReceiverName is used to identify this instance of the receiver.
	ReceiverName string `envconfig:"POD_NAME" default:"receiver-default" required:"true"`

	// EventFormat is used to verify the format of the received events
	EventFormat string `envconfig:"EVENT_FORMAT" default:"" required:"false"`

	// EnforceTLS is used to enforce TLS.
	EnforceTLS bool `envconfig:"ENFORCE_TLS" default:"false"`

	// OIDCAudience is the audience required for OIDC tokens reaching the receiver
	OIDCAudience string `envconfig:"OIDC_AUDIENCE" default:""`

	// ResponseWaitTime is the seconds to wait for the eventshub to write any response
	ResponseWaitTime int `envconfig:"RESPONSE_WAIT_TIME" default:"0" required:"false"`

	// Reply is used to define if the observer should reply back
	Reply bool `envconfig:"REPLY" default:"false" required:"false"`

	// The event type to use in the reply, if enabled
	ReplyEventType string `envconfig:"REPLY_EVENT_TYPE" default:"" required:"false"`

	// The event source to use in the reply, if enabled
	ReplyEventSource string `envconfig:"REPLY_EVENT_SOURCE" default:"" required:"false"`

	// The event data to use in the reply, if enabled
	ReplyEventData string `envconfig:"REPLY_EVENT_DATA" default:"" required:"false"`

	// This string to append in the data field in the reply, if enabled.
	// This will threat the data as text/plain field
	ReplyAppendData string `envconfig:"REPLY_APPEND_DATA" default:"" required:"false"`

	// If events should be dropped, specify the strategy here.
	SkipStrategy string `envconfig:"SKIP_ALGORITHM" default:"" required:"false"`

	// If events should be dropped according to Linear policy, this controls
	// how many events are dropped.
	SkipCounter uint64 `envconfig:"SKIP_COUNTER" default:"0" required:"false"`

	// If events should be dropped, specify the HTTP response code here.
	SkipResponseCode int `envconfig:"SKIP_RESPONSE_CODE" default:"409" required:"false"`

	// If events should be dropped, specify the HTTP response body here.
	SkipResponseBody string `envconfig:"SKIP_RESPONSE_BODY" default:"" required:"false"`

	// If events should be dropped, specify additional HTTP Headers to return in response.
	SkipResponseHeaders map[string]string `envconfig:"SKIP_RESPONSE_HEADERS" default:"" required:"false"`
}

func NewFromEnv(ctx context.Context, eventLogs *eventshub.EventLogs) *Receiver {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logging.FromContext(ctx).Fatal("Failed to process env var", err)
	}

	logging.FromContext(ctx).Infof("Receiver environment configuration: %+v", env)

	var replyFunc func(context.Context, http.ResponseWriter, eventshub.EventInfo)
	if env.Reply {
		logging.FromContext(ctx).Info("Receiver will reply with an event")
		replyFunc = ReplyTransformerFunc(env.ReplyEventType, env.ReplyEventSource, env.ReplyEventData, env.ReplyAppendData)
	} else {
		logging.FromContext(ctx).Info("Receiver won't reply with an event")
		replyFunc = NoOpReply
	}
	var counter *dropevents.CounterHandler

	if env.SkipStrategy != "" {
		counter = &dropevents.CounterHandler{
			Skipper: dropevents.SkipperAlgorithmWithCount(env.SkipStrategy, env.SkipCounter),
		}
	} else {
		counter = &dropevents.CounterHandler{
			// Don't skip anything, since count is 0. nop skipper.
			Skipper: dropevents.NoopSkipper,
		}
	}

	var responseWaitTime time.Duration
	if env.ResponseWaitTime != 0 {
		responseWaitTime = time.Duration(env.ResponseWaitTime) * time.Second
	}

	var expectedEventFormat *string
	if env.EventFormat != "" {
		expectedEventFormat = &env.EventFormat
	}

	return &Receiver{
		Name:                env.ReceiverName,
		EnforceTLS:          env.EnforceTLS,
		EventLogs:           eventLogs,
		ctx:                 ctx,
		replyFunc:           replyFunc,
		counter:             counter,
		responseWaitTime:    responseWaitTime,
		skipResponseCode:    env.SkipResponseCode,
		skipResponseBody:    env.SkipResponseBody,
		skipResponseHeaders: env.SkipResponseHeaders,
		oidcAudience:        env.OIDCAudience,
		kubeclient:          kubeclient.Get(ctx),
		expectedFormat:      expectedEventFormat,
	}
}

// Start will create the CloudEvents client and start listening for inbound
// HTTP requests. This is a is a blocking call.
func (o *Receiver) Start(ctx context.Context, handlerFuncs ...func(handler http.Handler) http.Handler) error {
	var handler http.Handler = o

	for _, dec := range handlerFuncs {
		handler = dec(handler)
	}

	server := &http.Server{Addr: ":8080", Handler: handler}
	serverTLS := &http.Server{
		Addr: ":8443",
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		Handler: handler,
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errors := make(chan error, 2)
	go func() {
		if err := server.ListenAndServe(); err != nil {
			errors <- fmt.Errorf("error while starting the HTTP server: %w", err)
			cancel()
		}
		defer server.Close()
	}()
	if o.EnforceTLS {
		go func() {
			if err := serverTLS.ListenAndServeTLS("/etc/tls/certificates/tls.crt", "/etc/tls/certificates/tls.key"); err != nil {
				errors <- fmt.Errorf("error while starting the HTTPS server: %w", err)
				cancel()
			}
		}()
		defer serverTLS.Close()
	}

	<-ctx.Done()
	close(errors)

	for err := range errors {
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *Receiver) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	// Special case probe events && readiness probe.
	if request.Method == http.MethodHead || request.URL.Path == "/health/ready" {
		code := http.StatusOK
		writer.WriteHeader(code)
		_, _ = writer.Write([]byte(http.StatusText(code)))
		return
	}

	var statusCode int
	var rejectErr error
	if o.EnforceTLS && !isTLS(request) {
		rejectErr = fmt.Errorf("failed to enforce TLS connection for request %s", request.URL.String())
		statusCode = http.StatusBadRequest
	}

	var oidcUser *authv1.UserInfo
	if o.oidcAudience != "" {
		var err error
		oidcUser, err = o.verifyJWT(request)
		if err != nil {
			rejectErr = err
			statusCode = http.StatusUnauthorized
		}
	}

	m := cloudeventshttp.NewMessageFromHttpRequest(request)
	defer m.Finish(nil)

	encoding := m.ReadEncoding()

	event, eventErr := cloudeventsbindings.ToEvent(context.TODO(), m)
	headers := make(http.Header)
	for k, v := range request.Header {
		if !strings.HasPrefix(k, "Ce-") {
			headers[k] = v
		}
	}
	// Host header is removed from the request.Header map by net/http
	if request.Host != "" {
		headers.Set("Host", request.Host)
	}

	errString := ""
	if rejectErr != nil {
		errString = rejectErr.Error()
	} else if eventErr != nil {
		errString = eventErr.Error()
	}

	shouldSkip := o.counter.Skip()
	var s uint64
	var kind eventshub.EventKind
	if shouldSkip || rejectErr != nil || !verifyFormat(o.expectedFormat, encoding) {
		kind = eventshub.EventRejected
		s = atomic.AddUint64(&o.dropSeq, 1)
	} else {
		kind = eventshub.EventReceived
		s = atomic.AddUint64(&o.seq, 1)
	}

	if shouldSkip {
		statusCode = o.skipResponseCode
	}

	eventInfo := eventshub.EventInfo{
		Error:        errString,
		Event:        event,
		HTTPHeaders:  headers,
		Origin:       request.RemoteAddr,
		Observer:     o.Name,
		Time:         time.Now(),
		Sequence:     s,
		Kind:         kind,
		Connection:   eventshub.TLSConnectionStateToConnection(request.TLS),
		StatusCode:   statusCode,
		OIDCUserInfo: oidcUser,
	}

	if err := o.EventLogs.Vent(eventInfo); err != nil {
		logging.FromContext(o.ctx).Fatalw("Error while venting the recorded event", zap.Error(err))
	}

	if o.responseWaitTime != 0 {
		logging.FromContext(o.ctx).Debugf("Waiting for %v before replying", o.responseWaitTime.String())
		time.Sleep(o.responseWaitTime)
	}

	if rejectErr != nil || !verifyFormat(o.expectedFormat, encoding) {
		for headerKey, headerValue := range o.skipResponseHeaders {
			writer.Header().Set(headerKey, headerValue)
		}

		writer.WriteHeader(statusCode)
	} else if shouldSkip {
		// Trigger a redelivery
		for headerKey, headerValue := range o.skipResponseHeaders {
			writer.Header().Set(headerKey, headerValue)
		}

		writer.WriteHeader(statusCode)
		_, _ = writer.Write([]byte(o.skipResponseBody))
	} else {
		o.replyFunc(o.ctx, writer, eventInfo)
	}
}

func (o *Receiver) getJWTFromRequest(request *http.Request) (string, error) {
	authHeader := request.Header.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("could not get Authorization header")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if len(token) == len(authHeader) {
		return "", fmt.Errorf("could not get Bearer token from header")
	}

	return token, nil
}

func (o *Receiver) verifyJWT(request *http.Request) (*authv1.UserInfo, error) {
	token, err := o.getJWTFromRequest(request)
	if err != nil {
		return nil, err
	}

	tokenReview, err := o.kubeclient.AuthenticationV1().TokenReviews().Create(o.ctx, &authv1.TokenReview{
		Spec: authv1.TokenReviewSpec{
			Token: token,
			Audiences: []string{
				o.oidcAudience,
			},
		},
	}, metav1.CreateOptions{})

	if err != nil {
		return nil, fmt.Errorf("could not get token review: %w", err)
	}

	if err := tokenReview.Status.Error; err != "" {
		return nil, fmt.Errorf("tokenReview error: %s", err)
	}

	if !tokenReview.Status.Authenticated {
		return nil, fmt.Errorf("user not authenticated")
	}

	return &tokenReview.Status.User, nil
}

func isTLS(request *http.Request) bool {
	return request.TLS != nil && request.TLS.HandshakeComplete && !eventshub.IsInsecureCipherSuite(request.TLS)
}

func verifyFormat(expected *string, actual cloudeventsbindings.Encoding) bool {
	// we don't care what format is, always return true
	if expected == nil {
		return true
	}

	switch *expected {
	case "binary":
		return actual == cloudeventsbindings.EncodingBinary
	case "json":
		return actual == cloudeventsbindings.EncodingStructured
	default:
		return false
	}
}
