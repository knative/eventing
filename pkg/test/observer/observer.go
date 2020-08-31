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
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/kelseyhightower/envconfig"
)

// Observer is the entry point for sinking events into the event log.
type Observer struct {
	// Name is the name of this Observer, used to filter if multiple observers.
	Name string
	// EventLogs is the list of EventLog implementors to vent observed events.
	EventLogs []EventLog
}

// New returns an observer that will vent observations to the list of provided
// EventLog instances. It will listen on :8080.
func New(eventLogs ...EventLog) *Observer {
	return &Observer{
		EventLogs: eventLogs,
	}
}

type envConfig struct {
	// ObserverName is used to identify this instance of the observer.
	ObserverName string `envconfig:"OBSERVER_NAME" default:"observer-default" required:"true"`
}

// Start will create the CloudEvents client and start listening for inbound
// HTTP requests. This is a is a blocking call.
func (o *Observer) Start(ctx context.Context) error {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		return err
	}
	o.Name = env.ObserverName

	ce, err := cloudevents.NewDefaultClient()
	if err != nil {
		return err
	}

	return ce.StartReceiver(ctx, o.onEvent)
}

func (o *Observer) onEvent(event cloudevents.Event) {
	obs := Observed{
		Event:    event,
		Origin:   "http://todo.origin", // TODO: we do not have this part at the moment, to get this info we will have to use http directly.
		Observer: o.Name,
		Time:     cloudevents.Timestamp{Time: time.Now()}.String(),
	}

	for _, el := range o.EventLogs {
		if el == nil {
			continue
		}
		// Observe the event.
		if err := el.Vent(obs); err != nil {
			fmt.Println(err)
		}
	}
}
