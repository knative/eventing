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
package adapter

import (
	"os"
	"testing"

	"github.com/kelseyhightower/envconfig"
)

type myEnvConfig struct {
	EnvConfig

	Mode string `envconfig:"MODE"`
}

func TestEnvConfig(t *testing.T) {
	os.Setenv("K_SINK", "http://sink")
	os.Setenv("NAMESPACE", "ns")
	os.Setenv("K_METRICS_CONFIG", "metrics")
	os.Setenv("K_LOGGING_CONFIG", "logging")
	os.Setenv("MODE", "mymode") // note: custom to this test impl

	var env myEnvConfig
	err := envconfig.Process("", &env)
	if err != nil {
		t.Errorf("Expected no error: %v", err)
	}

	if env.Mode != "mymode" {
		t.Errorf("Expected mode mymode, got: %s", env.Mode)
	}

	if env.Sink != "http://sink" {
		t.Errorf("Expected sinkURI http://sink, got: %s", env.Sink)
	}
}
