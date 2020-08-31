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

package http_vent

import (
	"context"
	"encoding/json"
	"log"
	"os"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/kelseyhightower/envconfig"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/test/observer"
)

type envConfig struct {
	Sink string `envconfig:"K_SINK"`
	// CEOverrides are the CloudEvents overrides to be applied to the outbound event.
	CEOverrides string `envconfig:"K_CE_OVERRIDES"`
}

func NewFromEnv(ctx context.Context) observer.EventLog {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Printf("[ERROR] Failed to process env var: %s", err)
		os.Exit(1)
	}

	if env.Sink == "" {
		return nil
	}

	p, err := cloudevents.NewHTTP(http.WithTarget(env.Sink))
	if err != nil {
		log.Printf("[ERROR] Failed to create cloudevents http protocol: %s", err)
		os.Exit(1)
	}

	ce, err := cloudevents.NewClient(p, cloudevents.WithTimeNow(), cloudevents.WithUUIDs())
	if err != nil {
		log.Printf("[ERROR] Failed to create cloudevents client: %s", err)
		os.Exit(1)
	}

	var ceOverrides *duckv1.CloudEventOverrides
	if len(env.CEOverrides) > 0 {
		overrides := duckv1.CloudEventOverrides{}
		err := json.Unmarshal([]byte(env.CEOverrides), &overrides)
		if err != nil {
			logging.FromContext(ctx).Errorf("[ERROR] Unparseable CloudEvents overrides %s: %v", env.CEOverrides, err)
			os.Exit(1)
		}
		ceOverrides = &overrides
	}

	return &cloudevent{out: ce, ceOverrides: ceOverrides}
}

type cloudevent struct {
	out         cloudevents.Client
	ceOverrides *duckv1.CloudEventOverrides
}

// Forwards the event.
func (w *cloudevent) Vent(observed observer.Observed) error {
	event := observed.Event

	if w.ceOverrides != nil && w.ceOverrides.Extensions != nil {
		for n, v := range w.ceOverrides.Extensions {
			event.SetExtension(n, v)
		}
	}

	if result := w.out.Send(context.Background(), event); !cloudevents.IsACK(result) {
		return result
	}

	return nil
}
