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

package observer

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	cloudeventsbindings "github.com/cloudevents/sdk-go/v2/binding"
	cloudeventshttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/test/lib/recordevents"
)

// Observer is the entry point for sinking events into the event log.
type Observer struct {

	// Name is the name of this Observer, used to filter if multiple observers.
	Name string
	// EventLogs is the list of EventLog implementors to vent observed events.
	EventLogs recordevents.EventLogs

	ctx       context.Context
	seq       uint64
	replyFunc func(context.Context, http.ResponseWriter, recordevents.EventInfo)
}

type envConfig struct {
	// ObserverName is used to identify this instance of the observer.
	ObserverName string `envconfig:"OBSERVER_NAME" default:"observer-default" required:"true"`

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
}

func NewFromEnv(ctx context.Context, eventLogs ...recordevents.EventLog) *Observer {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logging.FromContext(ctx).Fatal("Failed to process env var", err)
	}

	logging.FromContext(ctx).Infof("Observer environment configuration: %+v", env)

	var replyFunc func(context.Context, http.ResponseWriter, recordevents.EventInfo)
	if env.Reply {
		logging.FromContext(ctx).Info("Observer will reply with an event")
		replyFunc = ReplyTransformerFunc(env.ReplyEventType, env.ReplyEventSource, env.ReplyEventData, env.ReplyAppendData)
	} else {
		logging.FromContext(ctx).Info("Observer won't reply with an event")
		replyFunc = NoOpReply
	}

	return &Observer{
		Name:      env.ObserverName,
		EventLogs: eventLogs,
		ctx:       ctx,
		replyFunc: replyFunc,
	}
}

// Start will create the CloudEvents client and start listening for inbound
// HTTP requests. This is a is a blocking call.
func (o *Observer) Start(ctx context.Context, handlerFuncs ...func(handler http.Handler) http.Handler) error {
	var handler http.Handler = o

	for _, dec := range handlerFuncs {
		handler = dec(handler)
	}

	server := &http.Server{Addr: ":8080", Handler: handler}

	go func() {
		if err := server.ListenAndServe(); err != nil {
			logging.FromContext(ctx).Fatal("Error while starting the HTTP server", err)
		}
	}()

	<-ctx.Done()

	logging.FromContext(ctx).Info("Closing the HTTP server")

	return server.Close()
}

func (o *Observer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	m := cloudeventshttp.NewMessageFromHttpRequest(request)
	defer m.Finish(nil)

	event, eventErr := cloudeventsbindings.ToEvent(context.TODO(), m)
	header := request.Header

	eventErrStr := ""
	if eventErr != nil {
		eventErrStr = eventErr.Error()
	}
	eventInfo := recordevents.EventInfo{
		Error:       eventErrStr,
		Event:       event,
		HTTPHeaders: header,
		Origin:      request.RemoteAddr,
		Observer:    o.Name,
		Time:        time.Now(),
		Sequence:    atomic.AddUint64(&o.seq, 1),
	}
	err := o.EventLogs.Vent(eventInfo)
	if err != nil {
		logging.FromContext(o.ctx).Warnw("Error while venting the recorded event", zap.Error(err))
	}

	o.replyFunc(o.ctx, writer, eventInfo)
}
