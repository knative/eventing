package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync/atomic"

	cloudeventsbindings "github.com/cloudevents/sdk-go/v2/binding"
	cloudeventshttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.uber.org/zap"

	"knative.dev/eventing/pkg/tracing"
	"knative.dev/eventing/test/lib/events"
)

var seq uint64

func handle(writer http.ResponseWriter, request *http.Request) {
	receivedEvent := events.ReceivedEvent{}
	receivedEvent.Seq = atomic.AddUint64(&seq, 1)

	// Get message
	m := cloudeventshttp.NewMessageFromHttpRequest(request)
	defer m.Finish(nil)

	// Convert message to event
	receivedEvent.Event, receivedEvent.Err = cloudeventsbindings.ToEvent(context.TODO(), m)

	receivedEvent.AdditionalHeaders = make(map[string]string)
	for k, v := range request.Header {
		if !strings.HasPrefix(k, "Ce-") {
			receivedEvent.AdditionalHeaders[k] = v[0]
		}
	}

	b, err := json.Marshal(receivedEvent)
	if err != nil {
		// If that happen, then the test code is somehow wrong
		panic(err)
	}
	println(string(b))

	writer.WriteHeader(http.StatusAccepted)
}

func main() {
	if err := tracing.SetupStaticPublishing(zap.NewNop().Sugar(), "", tracing.AlwaysSample); err != nil {
		log.Fatalf("Unable to setup trace publishing: %v", err)
	}

	err := http.ListenAndServe(":8080", http.HandlerFunc(handle))
	if err != nil {
		panic(err)
	}
}
