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
	"os"
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
	os.Setenv("K_SINK", "http://sink")
	os.Setenv("NAMESPACE", "ns")
	os.Setenv("K_METRICS_CONFIG", "metrics")
	os.Setenv("K_LOGGING_CONFIG", "logging")
	os.Setenv("K_TRACING_CONFIG", "tracing")
	os.Setenv("K_LEADER_ELECTION_CONFIG", "leaderelection")
	os.Setenv("K_SINK_TIMEOUT", "999")
	os.Setenv("MODE", "mymode") // note: custom to this test impl

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

	if env.EnvSinkTimeout != "999" {
		t.Error("Expected env.EnvSinkTimeout to be 999, got:", env.EnvSinkTimeout)
	}
}

func TestEmptySinkTimeout(t *testing.T) {
	os.Setenv("K_SINK_TIMEOUT", "")
	defer func() {
		os.Unsetenv("K_SINK_TIMEOUT")
	}()

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

	os.Setenv("NAME", want)
	defer func() {
		os.Unsetenv("NAME")
	}()

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

	os.Setenv("NAMESPACE", want)
	defer func() {
		os.Unsetenv("NAMESPACE")
	}()

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

	os.Setenv("K_CE_OVERRIDES", string(wantJson))
	defer func() {
		os.Unsetenv("K_CE_OVERRIDES")
	}()

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
	os.Setenv("K_CE_OVERRIDES", "quack attack")
	defer func() {
		os.Unsetenv("K_CE_OVERRIDES")
	}()

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
	os.Setenv("K_COMPONENT", "Gotham")
	defer func() {
		os.Unsetenv("K_COMPONENT")
	}()

	want := new(kle.ComponentConfig)
	want.Buckets = 2
	want.Component = "Gotham"
	want.Identity = "Batman"
	want.LeaseDuration = 1 * time.Second
	want.RenewDeadline = 2 * time.Second
	want.RetryPeriod = 3 * time.Second

	wantJson, _ := LeaderElectionComponentConfigToJSON(want)

	fmt.Println(wantJson)

	os.Setenv("K_LEADER_ELECTION_CONFIG", wantJson)
	defer func() {
		os.Unsetenv("K_LEADER_ELECTION_CONFIG")
	}()

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
