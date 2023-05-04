/*
Copyright 2023 The Knative Authors

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

package forwarder

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cloudeventsbindings "github.com/cloudevents/sdk-go/v2/binding"
	"go.opencensus.io/trace"
	"go.uber.org/zap"

	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	cloudeventshttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/kelseyhightower/envconfig"
	"knative.dev/pkg/logging"

	"knative.dev/reconciler-test/pkg/eventshub"
)

// Forwarder is the entry point for sinking events into the event log.
type Forwarder struct {
	// Name is the name of this Forwarder.
	Name string

	// The current namespace.
	Namespace string `envconfig:"NAMESPACE" required:"true"`

	// Sink
	Sink string

	// EventLogs is the list of EventLogger implementors to vent observed events.
	EventLogs *eventshub.EventLogs

	ctx          context.Context
	handlerFuncs []eventshub.HandlerFunc
	clientOpts   []eventshub.ClientOption
	httpClient   *http.Client
}

type envConfig struct {
	// Name is used to identify this instance of the forwarder.
	Name string `envconfig:"NAME" default:"forwarder-default" required:"true"`

	// The current namespace.
	Namespace string `envconfig:"NAMESPACE" required:"true"`

	// Sink url for the message destination
	Sink string `envconfig:"SINK" required:"true"`
}

func NewFromEnv(ctx context.Context, eventLogs *eventshub.EventLogs, handlerFuncs []eventshub.HandlerFunc, clientOpts []eventshub.ClientOption) *Forwarder {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logging.FromContext(ctx).Fatal("Failed to process env var", err)
	}

	logging.FromContext(ctx).Infof("Forwarder environment configuration: %+v", env)

	return &Forwarder{
		Name:         env.Name,
		Namespace:    env.Namespace,
		Sink:         env.Sink,
		EventLogs:    eventLogs,
		ctx:          ctx,
		handlerFuncs: handlerFuncs,
		clientOpts:   clientOpts,
		httpClient:   &http.Client{},
	}
}

// Start will create the CloudEvents client and start listening for inbound
// HTTP requests. This is a blocking call.
func (o *Forwarder) Start(ctx context.Context) error {
	var handler http.Handler = o

	for _, opt := range o.clientOpts {
		if err := opt(o.httpClient); err != nil {
			return fmt.Errorf("unable to apply client option: %w", err)
		}
	}

	for _, dec := range o.handlerFuncs {
		handler = dec(handler)
	}

	server := &http.Server{Addr: ":8080", Handler: handler}

	var err error
	go func() {
		err = server.ListenAndServe()
	}()

	<-ctx.Done()

	if err != nil {
		return fmt.Errorf("error while starting the HTTP server: %w", err)
	}

	logging.FromContext(ctx).Info("Closing the HTTP server")

	return server.Close()
}

func (o *Forwarder) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	requestCtx, span := trace.StartSpan(request.Context(), "eventshub-forwarder")
	defer span.End()

	m := cloudeventshttp.NewMessageFromHttpRequest(request)
	defer m.Finish(nil)

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

	eventErrStr := ""
	if eventErr != nil {
		eventErrStr = eventErr.Error()
	}

	eventInfo := eventshub.EventInfo{
		Error:       eventErrStr,
		Event:       event,
		Observer:    o.Name,
		HTTPHeaders: headers,
		Origin:      request.RemoteAddr,
		Time:        time.Now(),
		Kind:        eventshub.EventReceived,
	}

	// Log the event that is being forwarded
	if err := o.EventLogs.Vent(eventInfo); err != nil {
		logging.FromContext(o.ctx).Fatalw("Error while venting the received event", zap.Error(err))
	}

	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, o.Sink, nil)
	if err != nil {
		logging.FromContext(o.ctx).Error("Cannot create the request: ", err)
	}
	err = cehttp.WriteRequest(requestCtx, m, req)
	if err != nil {
		logging.FromContext(o.ctx).Error("Cannot write the event: ", err)
	}

	eventString := "unknown"
	if event != nil {
		eventString = event.String()
	}
	span.AddAttributes(
		trace.StringAttribute("namespace", o.Namespace),
		trace.StringAttribute("event", eventString),
	)

	res, err := o.httpClient.Do(req)

	// Publish sent event info
	if err := o.EventLogs.Vent(o.sentInfo(event, req, err)); err != nil {
		logging.FromContext(o.ctx).Error("Cannot log forwarded event: ", err)
	}

	if err == nil {
		// Vent the response info
		if err := o.EventLogs.Vent(o.responseInfo(res, event)); err != nil {
			logging.FromContext(o.ctx).Error("Cannot log response for forwarded event: ", err)
		}
	}

	writer.WriteHeader(http.StatusAccepted)
}

func (o *Forwarder) sentInfo(event *cloudevents.Event, req *http.Request, err error) eventshub.EventInfo {
	var eventId string
	if event != nil {
		eventId = event.ID()
	}

	eventInfo := eventshub.EventInfo{
		Kind:     eventshub.EventSent,
		Origin:   o.Name,
		Observer: o.Name,
		Time:     time.Now(),
		SentId:   eventId,
	}

	sentHeaders := make(http.Header)
	for k, v := range req.Header {
		sentHeaders[k] = v
	}
	eventInfo.HTTPHeaders = sentHeaders

	if err != nil {
		eventInfo.Error = err.Error()
	} else {
		eventInfo.Event = event
	}

	return eventInfo
}

func (o *Forwarder) responseInfo(res *http.Response, event *cloudevents.Event) eventshub.EventInfo {
	var eventId string
	if event != nil {
		eventId = event.ID()
	}

	responseInfo := eventshub.EventInfo{
		Kind:        eventshub.EventResponse,
		HTTPHeaders: res.Header,
		Origin:      o.Sink,
		Observer:    o.Name,
		Time:        time.Now(),
		StatusCode:  res.StatusCode,
		SentId:      eventId,
	}

	responseMessage := cehttp.NewMessageFromHttpResponse(res)

	if responseMessage.ReadEncoding() == cloudeventsbindings.EncodingUnknown {
		body, err := ioutil.ReadAll(res.Body)

		if err != nil {
			responseInfo.Error = err.Error()
		} else {
			responseInfo.Body = body
		}
	} else {
		responseEvent, err := cloudeventsbindings.ToEvent(context.Background(), responseMessage)
		if err != nil {
			responseInfo.Error = err.Error()
		} else {
			responseInfo.Event = responseEvent
		}
	}
	return responseInfo
}
