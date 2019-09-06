/*
Copyright 2019 The Knative Authors

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
package utils

// TODO to move this to pkg
import (
	"encoding/json"
	"errors"
	"strconv"

	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
)

var zapLoggerConfig = "zap-logger-config"

// Base64ToMetricsOptions converts a json+base64 string of a
// metrics.ExporterOptions. Returns a non-nil metrics.ExporterOptions always.
func Base64ToMetricsOptions(base64 string) (*metrics.ExporterOptions, error) {
	var opts metrics.ExporterOptions
	if base64 == "" {
		return nil, errors.New("base64 metrics string is empty")
	}

	quoted64 := strconv.Quote(string(base64))

	var bytes []byte
	if err := json.Unmarshal([]byte(quoted64), &bytes); err != nil {
		return nil, err
	}

	if err := json.Unmarshal(bytes, &opts); err != nil {
		return nil, err
	}

	return &opts, nil
}

// MetricsOptionsToBase64 converts a metrics.ExporterOptions to a json+base64
// string.
func MetricsOptionsToBase64(opts *metrics.ExporterOptions) (string, error) {
	if opts == nil {
		return "", nil
	}

	jsonOpts, err := json.Marshal(opts)
	if err != nil {
		return "", err
	}
	// if we json.Marshal a []byte, we will get back a base64 encoded quoted string.
	base64Opts, err := json.Marshal(jsonOpts)
	if err != nil {
		return "", err
	}

	// Turn the base64 encoded []byte back into a string.
	base64, err := strconv.Unquote(string(base64Opts))
	if err != nil {
		return "", err
	}
	return base64, nil
}

// Base64ToLoggingConfig converts a json+base64 string of a logging.Config.
// Returns a non-nil logging.Config always.
func Base64ToLoggingConfig(base64 string) (*logging.Config, error) {
	if base64 == "" {
		return nil, errors.New("base64 logging string is empty")
	}

	quoted64 := strconv.Quote(string(base64))

	var bytes []byte
	if err := json.Unmarshal([]byte(quoted64), &bytes); err != nil {
		return nil, err
	}

	var configMap map[string]string
	if err := json.Unmarshal(bytes, &configMap); err != nil {
		return nil, err
	}

	cfg, err := logging.NewConfigFromMap(configMap)
	if err != nil {
		// Get the default config from logging package.
		if cfg, err = logging.NewConfigFromMap(map[string]string{}); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

// LoggingConfigToBase64 converts a logging.Config to a json+base64 string.
func LoggingConfigToBase64(cfg *logging.Config) (string, error) {
	if cfg == nil || cfg.LoggingConfig == "" {
		return "", nil
	}

	jsonCfg, err := json.Marshal(map[string]string{
		zapLoggerConfig: cfg.LoggingConfig,
	})
	if err != nil {
		return "", err
	}
	// if we json.Marshal a []byte, we will get back a base64 encoded quoted string.
	base64Cfg, err := json.Marshal(jsonCfg)
	if err != nil {
		return "", err
	}

	// Turn the base64 encoded []byte back into a string.
	base64, err := strconv.Unquote(string(base64Cfg))
	if err != nil {
		return "", err
	}
	return base64, nil
}
