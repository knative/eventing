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
	"github.com/prometheus/common/log"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/test/lib/recordevents"
)

// Observer is the entry point for sinking events into the event log.
type Observer struct {
	// Name is the name of this Observer, used to filter if multiple observers.
	Name string
	// EventLogs is the list of EventLog implementors to vent observed events.
	EventLogs recordevents.EventLogs

	seq uint64
}

// New returns an observer that will vent observations to the list of provided
// EventLog instances. It will listen on :8080.
func New(name string, eventLogs ...recordevents.EventLog) *Observer {
	return &Observer{
		Name:      name,
		EventLogs: eventLogs,
	}
}

type envConfig struct {
	// ObserverName is used to identify this instance of the observer.
	ObserverName string `envconfig:"OBSERVER_NAME" default:"observer-default" required:"true"`
}

func NewFromEnv(eventLogs ...recordevents.EventLog) *Observer {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatal("Failed to process env var", err)
	}

	return New(env.ObserverName, eventLogs...)
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
	err := o.EventLogs.Vent(recordevents.EventInfo{
		Error:       eventErrStr,
		Event:       event,
		HTTPHeaders: header,
		Origin:      request.RemoteAddr,
		Observer:    o.Name,
		Time:        time.Now(),
		Sequence:    atomic.AddUint64(&o.seq, 1),
	})
	if err != nil {
		log.Warn("Error while venting the recorded event", err)
	}

	writer.WriteHeader(http.StatusAccepted)
}
