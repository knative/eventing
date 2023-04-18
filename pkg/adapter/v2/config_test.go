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
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/kelseyhightower/envconfig"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	kle "knative.dev/pkg/leaderelection"
)

type myEnvConfig struct {
	EnvConfig

	Mode string `envconfig:"MODE"`
}

func TestEnvConfig(t *testing.T) {
	t.Setenv("K_SINK", "http://sink")
	t.Setenv("NAMESPACE", "ns")
	t.Setenv("K_METRICS_CONFIG", "metrics")
	t.Setenv("K_LOGGING_CONFIG", "logging")
	t.Setenv("K_TRACING_CONFIG", "tracing")
	t.Setenv("K_LEADER_ELECTION_CONFIG", "leaderelection")
	t.Setenv("K_SINK_TIMEOUT", "999")
	t.Setenv("MODE", "mymode") // note: custom to this test impl

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

	if env.EnvSinkTimeout != "999" {
		t.Error("Expected env.EnvSinkTimeout to be 999, got:", env.EnvSinkTimeout)
	}

	if env.CACerts != nil {
		t.Error("Expected CACerts to be nil")
	}
}

func TestEmptySinkTimeout(t *testing.T) {
	t.Setenv("K_SINK_TIMEOUT", "")

	var env myEnvConfig
	err := envconfig.Process("", &env)
	if err != nil {
		t.Error("Expected no error:", err)
	}

	if env.GetSinktimeout() != -1 {
		t.Error("Expected env.EnvSinkTimeout to be -1, got:", env.GetSinktimeout())
	}
}

func TestGetName(t *testing.T) {
	var env myEnvConfig
	err := envconfig.Process("", &env)
	if err != nil {
		t.Error("Expected no error:", err)
	}

	want := "adapter" // default.

	if got := env.GetName(); got != want {
		t.Errorf("Expected env.GetName() to be %q, got: %q", want, got)
	}
}

func TestGetName_Override(t *testing.T) {
	want := "custom-name"

	t.Setenv("NAME", want)

	var env myEnvConfig
	err := envconfig.Process("", &env)
	if err != nil {
		t.Error("Expected no error:", err)
	}

	if got := env.GetName(); got != want {
		t.Errorf("Expected env.GetName() to be %q, got: %q", want, got)
	}
}

func TestGetNamespace(t *testing.T) {
	var env myEnvConfig
	err := envconfig.Process("", &env)
	if err != nil {
		t.Error("Expected no error:", err)
	}

	want := "" // default.

	if got := env.GetNamespace(); got != want {
		t.Errorf("Expected env.GetNamespace() to be %q, got: %q", want, got)
	}
}

func TestGetNamespace_Override(t *testing.T) {
	want := "custom-namespace"

	t.Setenv("NAMESPACE", want)

	var env myEnvConfig
	err := envconfig.Process("", &env)
	if err != nil {
		t.Error("Expected no error:", err)
	}

	if got := env.GetNamespace(); got != want {
		t.Errorf("Expected env.GetNamespace() to be %q, got: %q", want, got)
	}
}

func TestGetCloudEventOverrides(t *testing.T) {
	want := new(duckv1.CloudEventOverrides)
	want.Extensions = map[string]string{"quack": "attack"}
	wantJson, _ := json.Marshal(want)

	t.Setenv("K_CE_OVERRIDES", string(wantJson))

	var env myEnvConfig
	err := envconfig.Process("", &env)
	if err != nil {
		t.Error("Expected no error:", err)
	}

	if got, err := env.GetCloudEventOverrides(); err != nil {
		t.Error("Expected no error:", err)
	} else if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("GetCloudEventOverrides (-want, +got) = %v", diff)
	}
}

func TestGetCloudEventOverrides_BadJson(t *testing.T) {
	t.Setenv("K_CE_OVERRIDES", "quack attack")

	var env myEnvConfig
	err := envconfig.Process("", &env)
	if err != nil {
		t.Error("Expected no error:", err)
	}

	if _, err := env.GetCloudEventOverrides(); err == nil {
		t.Error("Expected error")
	}
}

func TestGetLeaderElectionConfig(t *testing.T) {
	t.Setenv("K_COMPONENT", "Gotham")

	want := new(kle.ComponentConfig)
	want.Buckets = 2
	want.Component = "Gotham"
	want.Identity = "Batman"
	want.LeaseDuration = 1 * time.Second
	want.RenewDeadline = 2 * time.Second
	want.RetryPeriod = 3 * time.Second

	wantJson, _ := LeaderElectionComponentConfigToJSON(want)

	fmt.Println(wantJson)

	t.Setenv("K_LEADER_ELECTION_CONFIG", wantJson)

	var env myEnvConfig
	err := envconfig.Process("", &env)
	if err != nil {
		t.Error("Expected no error:", err)
	}

	if got, err := env.GetLeaderElectionConfig(); err != nil {
		t.Error("Expected no error:", err)
	} else if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("GetLeaderElectionConfig (-want, +got) = %v", diff)
	}
}

func TestCACerts(t *testing.T) {
	tt := []struct {
		value string
	}{
		{value: "foo"},
		{value: ""},
	}

	for _, tc := range tt {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("K_CA_CERTS", tc.value)

			var env myEnvConfig
			err := envconfig.Process("", &env)
			if err != nil {
				t.Error("Expected no error:", err)
			}

			if env.CACerts == nil {
				t.Error("Expected CACerts to be non nil")
			}
			if *env.CACerts != tc.value {
				t.Errorf("Expected CACerts to be %v, got %v", tc.value, *env.CACerts)
			}
		})
	}
}
