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

package test_images

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/tracing"
	"knative.dev/pkg/tracing/config"
)

func ParseHeaders(serializedHeaders string) http.Header {
	h := make(http.Header)
	for _, kv := range strings.Split(serializedHeaders, ",") {
		splitted := strings.Split(kv, "=")
		h.Set(splitted[0], splitted[1])
	}
	return h
}

// ParseDurationStr parses `durationStr` as number of seconds (not time.Duration string),
// if parsing fails, returns back default duration.
func ParseDurationStr(durationStr string, defaultDuration int) time.Duration {
	var duration time.Duration
	if d, err := strconv.Atoi(durationStr); err != nil {
		duration = time.Duration(defaultDuration) * time.Second
	} else {
		duration = time.Duration(d) * time.Second
	}
	return duration
}

const (
	ConfigTracingEnv = "K_CONFIG_TRACING"
	ConfigLoggingEnv = "K_CONFIG_LOGGING"
)

// ConfigureTracing can be used in test-images to configure tracing
func ConfigureTracing(logger *zap.SugaredLogger, serviceName string) error {
	tracingEnv := os.Getenv(ConfigTracingEnv)

	if tracingEnv == "" {
		return tracing.SetupStaticPublishing(logger, serviceName, config.NoopConfig())
	}

	conf, err := config.JSONToTracingConfig(tracingEnv)
	if err != nil {
		return err
	}

	return tracing.SetupStaticPublishing(logger, serviceName, conf)
}

// ConfigureTracing can be used in test-images to configure tracing
func ConfigureLogging(ctx context.Context, name string) context.Context {
	loggingEnv := os.Getenv(ConfigLoggingEnv)
	conf, err := logging.JSONToConfig(loggingEnv)
	if err != nil {
		logging.FromContext(ctx).Warn("Error while trying to read the config logging env: ", err)
		return ctx
	}
	l, _ := logging.NewLoggerFromConfig(conf, name)
	return logging.WithLogger(ctx, l)
}
