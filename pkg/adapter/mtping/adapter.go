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
	"os"
	"strconv"

	"knative.dev/pkg/controller"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/prometheus/common/log"
	"go.uber.org/zap"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/adapter/v2"
)

const (
	EnvNoShutdownAfter = "K_NO_SHUTDOWN_AFTER"
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
	logger := logging.FromContext(ctx)
	runner := NewCronJobsRunner(ceClient, kubeclient.Get(ctx), logging.FromContext(ctx))

	return &mtpingAdapter{
		logger: logger,
		runner: runner,
	}
}

// Start implements adapter.Adapter
func (a *mtpingAdapter) Start(ctx context.Context) error {
	ctrl := NewController(ctx, a.runner)

	a.logger.Info("Starting controllers...")
	go controller.StartAll(ctx, ctrl)

	defer a.runner.Stop()
	a.logger.Info("Starting job runner...")
	a.runner.Start(ctx.Done())

	a.logger.Infof("runner stopped")
	return nil
}

func GetNoShutDownAfterValue() int {
	str := os.Getenv(EnvNoShutdownAfter)
	if str != "" {
		second, err := strconv.Atoi(str)
		if err != nil || second < 0 || second > 59 {
			log.Warnf("%s environment value is invalid. It must be a integer between 0 and 59. (got %s)", EnvNoShutdownAfter, str)
		} else {
			return second
		}
	}
	return 55 // seems a reasonable default
}
