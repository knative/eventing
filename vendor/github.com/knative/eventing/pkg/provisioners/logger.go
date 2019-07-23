/*
 * Copyright 2018 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package provisioners

import (
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	provisionerLoggingComponent = "provisioner"
)

// NewLoggingConfig creates a static logging configuration appropriate for a
// provisioner. All logging levels are set to Info.
func NewLoggingConfig() *logging.Config {
	lc := &logging.Config{}
	lc.LoggingConfig = `{
		"level": "info",
		"development": false,
		"outputPaths": ["stdout"],
		"errorOutputPaths": ["stderr"],
		"encoding": "json",
		"encoderConfig": {
			"timeKey": "ts",
			"levelKey": "level",
			"nameKey": "logger",
			"callerKey": "caller",
			"messageKey": "msg",
			"stacktraceKey": "stacktrace",
			"lineEnding": "",
			"levelEncoder": "",
			"timeEncoder": "iso8601",
			"durationEncoder": "",
			"callerEncoder": ""
		}
	}`
	lc.LoggingLevel = make(map[string]zapcore.Level)
	lc.LoggingLevel[provisionerLoggingComponent] = zapcore.InfoLevel
	return lc
}

// NewProvisionerLoggerFromConfig creates a new zap logger for the provisioner component based
// on the provided configuration
func NewProvisionerLoggerFromConfig(config *logging.Config) *zap.SugaredLogger {
	logger, _ := logging.NewLoggerFromConfig(config, provisionerLoggingComponent)
	return logger
}
