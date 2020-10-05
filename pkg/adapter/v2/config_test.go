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
	os.Setenv("K_TRACING_CONFIG", "tracing")
	os.Setenv("K_LEADER_ELECTION_CONFIG", "leaderelection")
	os.Setenv("MODE", "mymode")        // note: custom to this test impl
	os.Setenv("K_SINK_TIMEOUT", "999") // note: custom to this test impl

	defer func() {
		os.Unsetenv("K_SINK")
		os.Unsetenv("NAMESPACE")
		os.Unsetenv("K_METRICS_CONFIG")
		os.Unsetenv("K_LOGGING_CONFIG")
		os.Unsetenv("K_TRACING_CONFIG")
		os.Unsetenv("K_LEADER_ELECTION_CONFIG")
		os.Unsetenv("MODE")
		os.Unsetenv("K_SINK_TIMEOUT")
	}()

	var env myEnvConfig
	err := envconfig.Process("", &env)
	if err != nil {
		t.Error("Expected no error:", err)
	}

	if env.Mode != "mymode" {
		t.Error("Expected mode mymode, got:", env.Mode)
	}

	if env.Sink != "http://sink" {
		t.Error("Expected sinkURI http://sink, got:", env.Sink)
	}

	if env.LeaderElectionConfigJson != "leaderelection" {
		t.Error("Expected LeaderElectionConfigJson leaderelection, got:", env.LeaderElectionConfigJson)
	}

	if sinkTimeout := GetSinkTimeout(nil); sinkTimeout != 999 {
		t.Error("Expected GetSinkTimeout to be 999, got:", sinkTimeout)
	}
	if env.EnvSinkTimeout != 999 {
		t.Error("Expected env.EnvSinkTimeout to be 999, got:", env.EnvSinkTimeout)
	}

}
