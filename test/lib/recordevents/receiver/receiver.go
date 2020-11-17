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
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"knative.dev/eventing/test/lib/dropevents"

	cloudeventsbindings "github.com/cloudevents/sdk-go/v2/binding"
	cloudeventshttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/test/lib/recordevents"
)

// Receiver is the entry point for sinking events into the event log.
type Receiver struct {
	// Name is the name of this Receiver, used to filter if multiple observers.
	Name string
	// EventLogs is the list of EventLogger implementors to vent observed events.
	EventLogs *recordevents.EventLogs

	ctx       context.Context
	seq       uint64
	dropSeq   uint64
	replyFunc func(context.Context, http.ResponseWriter, recordevents.EventInfo)
	counter   *dropevents.CounterHandler
}

type envConfig struct {
	// ReceiverName is used to identify this instance of the receiver.
	ReceiverName string `envconfig:"POD_NAME" default:"receiver-default" required:"true"`

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
}

func NewFromEnv(ctx context.Context, eventLogs *recordevents.EventLogs) *Receiver {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logging.FromContext(ctx).Fatal("Failed to process env var", err)
	}

	logging.FromContext(ctx).Infof("Receiver environment configuration: %+v", env)

	var replyFunc func(context.Context, http.ResponseWriter, recordevents.EventInfo)
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
			Skipper: dropevents.SkipperAlgorithmWithCount(dropevents.Sequence, 0),
		}
	}

	return &Receiver{
		Name:      env.ReceiverName,
		EventLogs: eventLogs,
		ctx:       ctx,
		replyFunc: replyFunc,
		counter:   counter,
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

func (o *Receiver) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
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

	shouldSkip := o.counter.Skip()
	var s uint64
	var kind recordevents.EventKind
	if shouldSkip {
		kind = recordevents.EventRejected
		s = atomic.AddUint64(&o.dropSeq, 1)
	} else {
		kind = recordevents.EventReceived
		s = atomic.AddUint64(&o.seq, 1)
	}

	eventInfo := recordevents.EventInfo{
		Error:       eventErrStr,
		Event:       event,
		HTTPHeaders: headers,
		Origin:      request.RemoteAddr,
		Observer:    o.Name,
		Time:        time.Now(),
		Sequence:    s,
		Kind:        kind,
	}

	if err := o.EventLogs.Vent(eventInfo); err != nil {
		logging.FromContext(o.ctx).Fatalw("Error while venting the recorded event", zap.Error(err))
	}

	if shouldSkip {
		// Trigger a redelivery
		writer.WriteHeader(http.StatusConflict)
	} else {
		o.replyFunc(o.ctx, writer, eventInfo)
	}
}
