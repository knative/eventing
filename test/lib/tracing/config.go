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
