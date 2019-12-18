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
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/robfig/cron"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/adapter"
	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/source"
)

type envConfig struct {
	adapter.EnvConfig

	// Environment variable container schedule.
	Schedule string `envconfig:"SCHEDULE" required:"true"`

	// Environment variable containing data.
	Data string `envconfig:"DATA" required:"true"`

	// Environment variable containing the name of the cron job.
	Name string `envconfig:"NAME" required:"true"`
}

// cronJobAdapter implements the Cron Job adapter to trigger a Sink.
type cronJobAdapter struct {
	// Schedule is a cron format string such as 0 * * * * or @hourly
	Schedule string

	// Data is the data to be posted to the target.
	Data string

	// Name is the name of the Cron Job.
	Name string

	// Namespace is the namespace of the Cron Job.
	Namespace string

	// client sends cloudevents.
	Client cloudevents.Client

	Reporter source.StatsReporter
}

const (
	resourceGroup = "cronjobsources.sources.eventing.knative.dev"
)

func NewEnvConfig() adapter.EnvConfigAccessor {
	return &envConfig{}
}

func NewAdapter(ctx context.Context, processed adapter.EnvConfigAccessor, ceClient cloudevents.Client, reporter source.StatsReporter) adapter.Adapter {
	env := processed.(*envConfig)

	return &cronJobAdapter{
		Schedule:  env.Schedule,
		Data:      env.Data,
		Name:      env.Name,
		Namespace: env.Namespace,
		Reporter:  reporter,
		Client:    ceClient,
	}
}

func (a *cronJobAdapter) Start(stopCh <-chan struct{}) error {
	sched, err := cron.ParseStandard(a.Schedule)
	if err != nil {
		return fmt.Errorf("Unparseable schedule %s: %v", a.Schedule, err)
	}

	c := cron.New()
	c.Schedule(sched, cron.FuncJob(a.cronTick))
	c.Start()
	<-stopCh
	c.Stop()
	return nil
}

func (a *cronJobAdapter) cronTick() {
	logger := logging.FromContext(context.TODO())

	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetType(sourcesv1alpha1.CronJobEventType)
	event.SetSource(sourcesv1alpha1.CronJobEventSource(a.Namespace, a.Name))
	event.SetData(message(a.Data))
	event.SetDataContentType(cloudevents.ApplicationJSON)
	reportArgs := &source.ReportArgs{
		Namespace:     a.Namespace,
		EventSource:   event.Source(),
		EventType:     event.Type(),
		Name:          a.Name,
		ResourceGroup: resourceGroup,
	}

	rctx, _, err := a.Client.Send(context.TODO(), event)
	rtctx := cloudevents.HTTPTransportContextFrom(rctx)
	if err != nil {
		logger.Error("failed to send cloudevent", zap.Error(err))
	}
	a.Reporter.ReportEventCount(reportArgs, rtctx.StatusCode)
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
