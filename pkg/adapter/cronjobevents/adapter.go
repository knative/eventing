/*
Copyright 2019 The Knative Authors

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

package cronjobevents

import (
	"context"
	"encoding/json"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"github.com/knative/eventing/pkg/kncloudevents"
	"github.com/knative/pkg/logging"
	"github.com/robfig/cron"
	"go.uber.org/zap"
)

const (
	eventType = "dev.knative.cronjob.event"
)

// TODO: this should be a k8s cron.

// Adapter implements the Cron Job adapter to trigger a Sink.
type Adapter struct {
	// Schedule is a cron format string such as 0 * * * * or @hourly
	Schedule string

	// Data is the data to be posted to the target.
	Data string

	// SinkURI is the URI messages will be forwarded on to.
	SinkURI string

	// client sends cloudevents.
	client client.Client
}

// Initialize cloudevent client
func (a *Adapter) initClient() error {
	if a.client == nil {
		var err error
		if a.client, err = kncloudevents.NewDefaultClient(a.SinkURI); err != nil {
			return err
		}
	}
	return nil
}

func (a *Adapter) Start(ctx context.Context, stopCh <-chan struct{}) error {
	logger := logging.FromContext(ctx)

	sched, err := cron.ParseStandard(a.Schedule)
	if err != nil {
		logger.Error("Unparseable schedule: ", a.Schedule, zap.Error(err))
		return err
	}

	if err = a.initClient(); err != nil {
		logger.Error("Failed to create cloudevent client", zap.Error(err))
		return err
	}

	c := cron.New()
	c.Schedule(sched, cron.FuncJob(a.cronTick))
	c.Start()
	<-stopCh
	c.Stop()
	logger.Info("Shutting down.")
	return nil
}

func (a *Adapter) cronTick() {
	logger := logging.FromContext(context.TODO())

	event := cloudevents.Event{
		Context: cloudevents.EventContextV02{
			Type:   eventType,
			Source: *types.ParseURLRef("/CronJob"),
		}.AsV02(),
		Data: message(a.Data),
	}
	if _, err := a.client.Send(context.TODO(), event); err != nil {
		logger.Error("failed to send cloudevent", err)
	}
}

type Message struct {
	Body string `json:"body"`
}

func message(body string) interface{} {
	// try to marshal the body into an interface.
	var objmap map[string]*json.RawMessage
	if err := json.Unmarshal([]byte(body), &objmap); err != nil {
		//default to a wrapped message.
		return Message{Body: body}
	}
	return objmap
}
