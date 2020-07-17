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

package mtping

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/adapter/v2"
)

// mtpingAdapter implements the PingSource mt adapter to sinks
type mtpingAdapter struct {
	logger *zap.SugaredLogger
	runner *cronJobsRunner
}

func NewEnvConfig() adapter.EnvConfigAccessor {
	return &adapter.EnvConfig{}
}

func NewAdapter(ctx context.Context, _ adapter.EnvConfigAccessor, ceClient cloudevents.Client) adapter.Adapter {
	runner := NewCronJobsRunner(ceClient, kubeclient.Get(ctx), logging.FromContext(ctx))

	cmw := adapter.ConfigMapWatcherFromContext(ctx)
	cmw.Watch("config-pingsource-mt-adapter", runner.updateFromConfigMap)

	return &mtpingAdapter{
		logger: logging.FromContext(ctx),
		runner: runner,
	}
}

// Start implements adapter.Adapter
func (a *mtpingAdapter) Start(ctx context.Context) error {
	a.logger.Info("Starting job runner...")
	if err := a.runner.Start(ctx.Done()); err != nil {
		return err
	}

	a.logger.Infof("runner stopped")
	return nil
}
