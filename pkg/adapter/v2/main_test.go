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

package adapter

import (
	"context"
	"os"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.opencensus.io/stats/view"
	"knative.dev/pkg/metrics"
)

type myAdapter struct{}

func TestMainWithContext(t *testing.T) {
	os.Setenv("K_SINK", "http://sink")
	os.Setenv("NAMESPACE", "ns")
	os.Setenv("K_METRICS_CONFIG", "metrics")
	os.Setenv("K_LOGGING_CONFIG", "logging")
	os.Setenv("MODE", "mymode")

	ctx, cancel := context.WithCancel(context.TODO())

	MainWithContext(ctx,
		"mycomponent",
		func() EnvConfigAccessor { return &myEnvConfig{} },
		func(ctx context.Context, processed EnvConfigAccessor, client cloudevents.Client) Adapter {
			env := processed.(*myEnvConfig)
			if env.Mode != "mymode" {
				t.Errorf("Expected mode mymode, got: %s", env.Mode)
			}

			if env.Sink != "http://sink" {
				t.Errorf("Expected sinkURI http://sink, got: %s", env.Sink)
			}
			return &myAdapter{}
		})

	cancel()

	defer view.Unregister(metrics.NewMemStatsAll().DefaultViews()...)
}

func (m *myAdapter) Start(stopCh <-chan struct{}) error {
	return nil
}
