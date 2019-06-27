/*
Copyright 2018 The Knative Authors

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

package provisioners

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	_ "knative.dev/pkg/system/testing"
)

func TestNewLoggingConfig(t *testing.T) {
	config := NewLoggingConfig()
	expected := map[string]zapcore.Level{
		"provisioner": zap.InfoLevel,
	}
	actual := config.LoggingLevel
	if cmp.Diff(expected, actual) != "" {
		t.Errorf("%s expected: %+v got: %+v", "Logging level", expected, actual)
	}
}

func TestNewBusLoggerFromConfig(t *testing.T) {
	config := NewLoggingConfig()
	logger := NewProvisionerLoggerFromConfig(config)
	expected := true
	actual := logger.Desugar().Core().Enabled(zap.InfoLevel)
	if expected != actual {
		t.Errorf("%s expected: %+v got: %+v", "Logging level", expected, actual)
	}
}
