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

package source

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing/pkg/observability"
	o11yconfigmap "knative.dev/eventing/pkg/observability/configmap"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/logging"
)

const (
	EnvLoggingCfg       = "K_LOGGING_CONFIG"
	EnvObservabilityCfg = "K_OBSERVABILITY_CONFIG"
)

type ConfigAccessor interface {
	ToEnvVars() []corev1.EnvVar
	LoggingConfig() *logging.Config
	ObservabilityConfig() *observability.Config
}

var _ ConfigAccessor = (*ConfigWatcher)(nil)

// ConfigWatcher keeps track of logging, metrics and tracing configurations by
// watching corresponding ConfigMaps.
type ConfigWatcher struct {
	logger *zap.SugaredLogger

	component string

	// configurations remain nil if disabled
	loggingCfg       *logging.Config
	observabilityCfg *observability.Config
}

// configWatcherOption is a function option for ConfigWatchers.
type configWatcherOption func(*ConfigWatcher, configmap.Watcher)

// WatchConfigurations returns a ConfigWatcher initialized with the given
// options. If no option is passed, the ConfigWatcher observes ConfigMaps for
// logging, and observability.
func WatchConfigurations(loggingCtx context.Context, component string,
	cmw configmap.Watcher, opts ...configWatcherOption) *ConfigWatcher {

	cw := &ConfigWatcher{
		logger:    logging.FromContext(loggingCtx),
		component: component,
	}

	if len(opts) == 0 {
		WithLogging(cw, cmw)
		WithObservability(cw, cmw)

	} else {
		for _, opt := range opts {
			opt(cw, cmw)
		}
	}

	return cw
}

// WithLogging observes a logging ConfigMap.
func WithLogging(cw *ConfigWatcher, cmw configmap.Watcher) {
	cw.loggingCfg = &logging.Config{}
	watchConfigMap(cmw, logging.ConfigMapName(), cw.updateFromLoggingConfigMap)
}

func WithObservability(cw *ConfigWatcher, cmw configmap.Watcher) {
	cw.observabilityCfg = &observability.Config{}
	watchConfigMap(cmw, o11yconfigmap.Name(), cw.updateFromObservabilityConfigMap)
}

func watchConfigMap(cmw configmap.Watcher, cmName string, obs configmap.Observer) {
	if dcmw, ok := cmw.(configmap.DefaultingWatcher); ok {
		dcmw.WatchWithDefault(corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: cmName},
			Data:       map[string]string{},
		}, obs)

	} else {
		cmw.Watch(cmName, obs)
	}
}

// LoggingConfig returns the logging configuration from the ConfigWatcher.
func (cw *ConfigWatcher) LoggingConfig() *logging.Config {
	if cw == nil {
		return nil
	}
	return cw.loggingCfg
}

func (cw *ConfigWatcher) updateFromLoggingConfigMap(cfg *corev1.ConfigMap) {
	if cfg == nil {
		return
	}

	delete(cfg.Data, "_example")

	loggingCfg, err := logging.NewConfigFromConfigMap(cfg)
	if err != nil {
		cw.logger.Warnw("failed to create logging config from ConfigMap", zap.String("cfg.Name", cfg.Name))
		return
	}

	cw.loggingCfg = loggingCfg

	cw.logger.Debugw("Updated logging config from ConfigMap", zap.Any("ConfigMap", cfg))
}

func (cw *ConfigWatcher) ObservabilityConfig() *observability.Config {
	if cw == nil {
		return nil
	}

	return cw.observabilityCfg
}

func (cw *ConfigWatcher) updateFromObservabilityConfigMap(cfg *corev1.ConfigMap) {
	if cfg == nil {
		return
	}

	obsCfg, err := o11yconfigmap.Parse(cfg)
	if err != nil {
		cw.logger.Warnw("failed to create observability config from ConfigMap", zap.String("cfg.Name", cfg.Name))
		return
	}

	cw.observabilityCfg = obsCfg

	cw.logger.Debugw("Updated observability config from ConfigMap", zap.Any("ConfigMap", cfg))
}

// ToEnvVars serializes the contents of the ConfigWatcher to individual
// environment variables.
func (cw *ConfigWatcher) ToEnvVars() []corev1.EnvVar {
	envs := make([]corev1.EnvVar, 0, 3)

	envs = maybeAppendEnvVar(envs, cw.loggingConfigEnvVar(), cw.LoggingConfig() != nil)
	envs = maybeAppendEnvVar(envs, cw.observabilityConfigEnvVar(), cw.ObservabilityConfig() != nil)

	return envs
}

// maybeAppendEnvVar appends an EnvVar only if the condition boolean is true.
func maybeAppendEnvVar(envs []corev1.EnvVar, env corev1.EnvVar, cond bool) []corev1.EnvVar {
	if !cond {
		return envs
	}

	return append(envs, env)
}

// loggingConfigEnvVar returns an EnvVar containing the serialized logging
// configuration from the ConfigWatcher.
func (cw *ConfigWatcher) loggingConfigEnvVar() corev1.EnvVar {
	logCfg := cw.LoggingConfig()
	if logCfg == nil {
		logCfg = &logging.Config{}
	}

	if lvl, hasLogLvl := logCfg.LoggingLevel[cw.component]; hasLogLvl {
		newLogCfg, err := overrideLoggingLevel(logCfg, lvl)
		if err != nil {
			cw.logger.With(zap.Error(err)).Warnf("Failed to apply logging level %q to logging config", lvl)
		} else {
			logCfg = newLogCfg
		}
	}

	cfg, err := logging.ConfigToJSON(logCfg)
	if err != nil {
		cw.logger.Warnw("Error while serializing logging config", zap.Error(err))
	}

	return corev1.EnvVar{
		Name:  EnvLoggingCfg,
		Value: cfg,
	}
}

// loggingConfigEnvVar returns an EnvVar containing the serialized logging
// configuration from the ConfigWatcher.
func (cw *ConfigWatcher) observabilityConfigEnvVar() corev1.EnvVar {
	obsCfg := cw.ObservabilityConfig()
	if obsCfg == nil {
		obsCfg = &observability.Config{}
	}

	cfg, err := json.Marshal(obsCfg)
	if err != nil {
		cw.logger.Warnw("Error while serializing observability config", zap.Error(err))
	}

	return corev1.EnvVar{
		Name:  EnvObservabilityCfg,
		Value: string(cfg),
	}
}

// overrideLoggingLevel returns cfg with the given logging level applied.
func overrideLoggingLevel(cfg *logging.Config, lvl zapcore.Level) (*logging.Config, error) {
	tmpCfg := &zapConfig{}
	if err := json.Unmarshal([]byte(cfg.LoggingConfig), tmpCfg); err != nil {
		return nil, fmt.Errorf("deserializing logging config from ConfigMap: %w", err)
	}

	tmpCfg.Level = zap.NewAtomicLevelAt(lvl)

	b, err := json.Marshal(tmpCfg)
	if err != nil {
		return nil, fmt.Errorf("serializing logging config with logging level applied: %w", err)
	}

	cfg.LoggingConfig = string(b)

	return cfg, nil
}

// EmptyVarsGenerator generates empty env vars. Intended to be used in tests.
type EmptyVarsGenerator struct {
	ConfigAccessor
}

var _ ConfigAccessor = (*EmptyVarsGenerator)(nil)

func (g *EmptyVarsGenerator) ToEnvVars() []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: EnvLoggingCfg},
		{Name: EnvObservabilityCfg},
	}
}

// zapConfig is a representation of a zap.Config that can be both unmarshaled
// from JSON and marshaled to JSON again, unlike the zap.Config type which
// contains func fields that can be unmarshaled but not marshaled. Those fields
// are shadowed here to prevent marshaling errors.
type zapConfig struct {
	zap.Config

	EncoderConfig struct {
		zapcore.EncoderConfig

		EncodeLevel    string `json:"levelEncoder" yaml:"levelEncoder"`
		EncodeTime     string `json:"timeEncoder" yaml:"timeEncoder"`
		EncodeDuration string `json:"durationEncoder" yaml:"durationEncoder"`
		EncodeCaller   string `json:"callerEncoder" yaml:"callerEncoder"`
		EncodeName     string `json:"nameEncoder" yaml:"nameEncoder"`
	} `json:"encoderConfig" yaml:"encoderConfig"`
}
