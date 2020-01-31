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
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"os"

	cloudevents "github.com/cloudevents/sdk-go"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/adapter"
	sourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/source"
)

type envConfig struct {
	adapter.EnvConfig

	// Environment variable containing data.
	Data string `envconfig:"DATA" required:"true"`

	// Environment variable containing the name of the adapter.
	Name string `envconfig:"NAME" required:"true"`

	// Environment variable containing the name of the adapter.
	CEOverrides string `envconfig:"K_CE_OVERRIDES"`
}

// pingAdapter implements the PingSource adapter to trigger a Sink.
type pingAdapter struct {
	// Data is the data to be posted to the target.
	Data string

	// Name is the name of the adapter.
	Name string

	// Namespace is the namespace of the adapter.
	Namespace string

	// client sends cloudevents.
	Client cloudevents.Client

	Reporter source.StatsReporter

	KCEOverrides        string
	CloudEventOverrides *duckv1.CloudEventOverrides
}

const (
	resourceGroup = "pingsources.sources.knative.dev"
)

func NewEnvConfig() adapter.EnvConfigAccessor {
	return &envConfig{}
}

func NewAdapter(ctx context.Context, processed adapter.EnvConfigAccessor, ceClient cloudevents.Client, reporter source.StatsReporter) adapter.Adapter {
	env := processed.(*envConfig)

	return &pingAdapter{
		Data:         env.Data,
		Name:         env.Name,
		Namespace:    env.Namespace,
		Reporter:     reporter,
		Client:       ceClient,
		KCEOverrides: env.CEOverrides,
	}
}

func (a *pingAdapter) Start(stopCh <-chan struct{}) error {
	if len(a.KCEOverrides) > 0 {
		overrides := duckv1.CloudEventOverrides{}
		err := json.Unmarshal([]byte(a.KCEOverrides), &overrides)
		if err != nil {
			return fmt.Errorf("Unparseable CloudEvents overrides %s: %v", a.KCEOverrides, err)
		}
		a.CloudEventOverrides = &overrides
	}

	a.cronTick()

	os.Exit(0)
	return nil
}

func (a *pingAdapter) cronTick() {
	logger := logging.FromContext(context.TODO())

	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetType(sourcesv1alpha1.PingSourceEventType)
	event.SetSource(sourcesv1alpha1.PingSourceSource(a.Namespace, a.Name))
	if err := event.SetData(message(a.Data)); err != nil {
		logger.Error("failed to set cloudevents data", zap.Error(err))
	}
	event.SetDataContentType(cloudevents.ApplicationJSON)

	if a.CloudEventOverrides != nil && a.CloudEventOverrides.Extensions != nil {
		for n, v := range a.CloudEventOverrides.Extensions {
			event.SetExtension(n, v)
		}
	}

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
