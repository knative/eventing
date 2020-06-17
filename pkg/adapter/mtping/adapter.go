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
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/adapter/v2"
)

// mtpingAdapter implements the PingSource mt adapter to
type mtpingAdapter struct {
	logger *zap.SugaredLogger
	client cloudevents.Client
}

func NewEnvConfig() adapter.EnvConfigAccessor {
	return &adapter.EnvConfig{}
}

func NewAdapter(ctx context.Context, _ adapter.EnvConfigAccessor, ceClient cloudevents.Client) adapter.Adapter {
	return &mtpingAdapter{
		logger: logging.FromContext(ctx),
		client: ceClient,
	}
}

// Start implements adapter.Adapter
func (a *mtpingAdapter) Start(ctx context.Context) error {
	runner := NewCronJobsRunner(a.client, a.logger)

	ctrl := NewController(ctx, runner)

	a.logger.Info("Starting controllers...")
	go controller.StartAll(ctx, ctrl)

	a.logger.Info("Starting job runner...")
	runner.Start(ctx.Done())

	a.logger.Infof("controller and runner stopped")
	return nil
}
