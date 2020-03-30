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

package ping

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/robfig/cron"
	"go.uber.org/zap"
	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/adapter/v2"
)

type envConfig struct {
	adapter.EnvConfig

	// Environment variable container schedule.
	Schedule string `envconfig:"SCHEDULE" required:"true"`

	// Environment variable containing data.
	Data string `envconfig:"DATA" required:"true"`
}

// pingAdapter implements the PingSource adapter to trigger a Sink.
type pingAdapter struct {
	// Schedule is a cron format string such as 0 * * * * or @hourly
	Schedule string

	// Data is the data to be posted to the target.
	Data string

	// Name is the name of the adapter.
	Name string

	// Namespace is the namespace of the adapter.
	Namespace string

	// client sends cloudevents.
	Client cloudevents.Client
}

func init() {
	_ = os.Setenv("K_RESOURCE_GROUP", "pingsources.sources.knative.dev")
}

func NewEnvConfig() adapter.EnvConfigAccessor {
	return &envConfig{}
}

func NewAdapter(ctx context.Context, processed adapter.EnvConfigAccessor, ceClient cloudevents.Client) adapter.Adapter {
	env := processed.(*envConfig)

	return &pingAdapter{
		Schedule:  env.Schedule,
		Data:      env.Data,
		Name:      env.Name,
		Namespace: env.Namespace,
		Client:    ceClient,
	}
}

func (a *pingAdapter) Start(stopCh <-chan struct{}) error {
	sched, err := cron.ParseStandard(a.Schedule)
	if err != nil {
		return fmt.Errorf("unparseable schedule %s: %v", a.Schedule, err)
	}

	c := cron.New()
	c.Schedule(sched, cron.FuncJob(a.cronTick))
	c.Start()
	<-stopCh
	c.Stop()
	return nil
}

func (a *pingAdapter) cronTick() {
	ctx := context.Background()
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetType(sourcesv1alpha1.PingSourceEventType)
	event.SetSource(sourcesv1alpha1.PingSourceSource(a.Namespace, a.Name))
	if err := event.SetData(cloudevents.ApplicationJSON, message(a.Data)); err != nil {
		logging.FromContext(ctx).Errorw("ping failed to set event data", zap.Error(err))
	}

	if err := a.Client.Send(ctx, event); err != nil {
		logging.FromContext(ctx).Errorw("ping failed to send cloudevent", zap.Error(err))
	}
}

type Message struct {
	Body string `json:"body"`
}

func message(body string) interface{} {
	// try to marshal the body into an interface.
	var obj map[string]*json.RawMessage
	if err := json.Unmarshal([]byte(body), &obj); err != nil {
		//default to a wrapped message.
		return Message{Body: body}
	}
	return obj
}
