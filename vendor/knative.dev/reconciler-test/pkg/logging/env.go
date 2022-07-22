/*
Copyright 2022 The Knative Authors
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

package logging

import (
	"context"
	"os"
	"strings"

	"go.uber.org/zap/zapcore"
	"knative.dev/pkg/logging"
)

// LevelFromEnvironment returns a zap level, based on the environment variable
// TEST_LOGGER_LEVEL.
func LevelFromEnvironment(ctx context.Context) zapcore.Level {
	const key = "TEST_LOGGER_LEVEL"
	levelStr := os.Getenv(key)
	if levelStr == "" {
		levelStr = zapcore.DebugLevel.String()
	}
	levelStr = strings.ToLower(levelStr)
	level := zapcore.DebugLevel
	if err := level.UnmarshalText([]byte(levelStr)); err != nil {
		logging.FromContext(ctx).
			Fatalf("Invalid level given as %s: %+v, %+v", key, levelStr, err)
	}
	return level
}
