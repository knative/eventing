package observer

import (
	"context"
	"net/http"
	"time"

	cloudeventsbindings "github.com/cloudevents/sdk-go/v2/binding"
	cloudeventshttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/common/log"

	"knative.dev/eventing/test/lib/recordevents"
)

// Observer is the entry point for sinking events into the event log.
type Observer struct {
	// Name is the name of this Observer, used to filter if multiple observers.
	Name string
	// EventLogs is the list of EventLog implementors to vent observed events.
	EventLogs []recordevents.EventLog
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
		log.Fatalf("[ERROR] Failed to process env var: %s", err)
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

	return http.ListenAndServe(":8080", handler)
}

func (o *Observer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	m := cloudeventshttp.NewMessageFromHttpRequest(request)
	defer m.Finish(nil)

	event, eventErr := cloudeventsbindings.ToEvent(context.TODO(), m)
	header := request.Header

	for _, el := range o.EventLogs {
		eventErrStr := ""
		if eventErr != nil {
			eventErrStr = eventErr.Error()
		}
		err := el.Vent(recordevents.EventInfo{
			Error:       eventErrStr,
			Event:       event,
			HTTPHeaders: header,
			Origin:      request.RemoteAddr,
			Observer:    "",
			Time:        time.Now(),
		})
		if err != nil {
			log.Warn("Error while venting the recorded event %s", err)
		}
	}

	writer.WriteHeader(http.StatusAccepted)
}
