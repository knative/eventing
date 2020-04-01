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

package jobrunner

import (
	"context"
	"encoding/json"

	"github.com/robfig/cron"
	"go.uber.org/zap"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	kncloudevents "knative.dev/eventing/pkg/adapter/v2"
	sourcesv1alpha2 "knative.dev/eventing/pkg/apis/sources/v1alpha2"
)

type cronJobsRunner struct {
	// The cron job runner
	cron cron.Cron

	// client sends cloudevents.
	Client cloudevents.Client

	// Where to send logs
	Logger *zap.SugaredLogger
}

const (
	resourceGroup = "pingsources.sources.knative.dev"
)

func NewCronJobsRunner(ceClient cloudevents.Client, logger *zap.SugaredLogger) *cronJobsRunner {
	return &cronJobsRunner{
		cron: *cron.New(),

		Client: ceClient,
		Logger: logger,
	}
}

func (a *cronJobsRunner) AddSchedule(namespace, name, spec, data, sink string) (cron.EntryID, error) {
	event := cloudevents.NewEvent()
	event.SetType(sourcesv1alpha2.PingSourceEventType)
	event.SetSource(sourcesv1alpha2.PingSourceSource(namespace, name))
	event.SetData(cloudevents.ApplicationJSON, message(data))

	ctx := context.Background()
	ctx = cloudevents.ContextWithTarget(ctx, sink)

	metricTag := &kncloudevents.MetricTag{
		Namespace:     namespace,
		Name:          name,
		ResourceGroup: resourceGroup,
	}
	ctx = kncloudevents.ContextWithMetricTag(ctx, metricTag)

	return a.cron.AddFunc(spec, a.cronTick(ctx, event))
}

func (a *cronJobsRunner) RemoveSchedule(id cron.EntryID) {
	a.cron.Remove(id)
}

func (a *cronJobsRunner) Start(stopCh <-chan struct{}) error {
	a.cron.Start()
	<-stopCh
	a.cron.Stop()
	return nil
}

func (a *cronJobsRunner) cronTick(ctx context.Context, event cloudevents.Event) func() {
	return func() {
		if result := a.Client.Send(ctx, event); !cloudevents.IsACK(result) {
			// TODO: at least retries
			a.Logger.Error("failed to send cloudevent", zap.Any("result", result))
		}
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
