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

package tracing

import (
	"os"

	"go.uber.org/zap"
	tracingconfig "knative.dev/pkg/tracing/config"

	"knative.dev/eventing/pkg/tracing"
)

const ConfigTracingEnv = "K_CONFIG_TRACING"

// ConfigureTracing can be used in test-images to configure tracing
func ConfigureTracing(logger *zap.SugaredLogger, serviceName string) error {
	tracingEnv := os.Getenv(ConfigTracingEnv)

	if tracingEnv == "" {
		return tracing.SetupStaticPublishing(logger, serviceName, tracing.AlwaysSample)
	}

	conf, err := tracingconfig.JsonToTracingConfig(tracingEnv)
	if err != nil {
		return err
	}

	return tracing.SetupStaticPublishing(logger, serviceName, conf)
}
