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

package main

import (
	"knative.dev/pkg/signals"

	"knative.dev/eventing/pkg/adapter/mtping"
	"knative.dev/eventing/pkg/adapter/v2"
)

const (
	component = "pingsource-mt-adapter"
)

func main() {
	sctx := signals.NewContext()

	// When cancelling the adapter to close to the minute, there is
	// a risk of losing events due to either the delay of starting a new pod
	// or for the passive pod to become active (when HA is enabled and replicas > 1).
	// So when receiving a SIGTEM signal, delay the cancellation of the adapter,
	// which under the cover delays the release of the lease.
	ctx := mtping.NewDelayingContext(sctx, mtping.GetNoShutDownAfterValue())

	ctx = adapter.WithController(ctx, mtping.NewController)
	ctx = adapter.WithHAEnabled(ctx)

	// The adapter constructor for PingSource uses sets watchs on ConfigMaps to
	// dynamically configure observability, profiling and CloudEvents reporting.
	ctx = adapter.WithConfigWatcherEnabled(ctx)
	ctx = adapter.WithConfiguratorOptions(ctx, []adapter.ConfiguratorOption{
		adapter.WithLoggerConfigurator(adapter.NewLoggerConfiguratorFromConfigMap(component)),
		adapter.WithMetricsExporterConfigurator(adapter.NewMetricsExporterConfiguratorFromConfigMap(component)),
		adapter.WithTracingConfigurator(adapter.NewTracingConfiguratorFromConfigMap()),
		adapter.WithProfilerConfigurator(adapter.NewProfilerConfiguratorFromConfigMap()),
		adapter.WithCloudEventsStatusReporterConfigurator(adapter.NewCloudEventsReporterConfiguratorFromConfigMap()),
	})

	adapter.MainWithContext(ctx, component, mtping.NewEnvConfig, mtping.NewAdapter)
}
