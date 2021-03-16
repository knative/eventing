// +build e2e

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

package rekt

import (
	"flag"
	"log"
	"os"
	"testing"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"knative.dev/pkg/injection"
	_ "knative.dev/pkg/system/testing"

	"knative.dev/reconciler-test/pkg/environment"
)

// global is the singleton instance of GlobalEnvironment. It is used to parse
// the testing config for the test run. The config will specify the cluster
// config as well as the parsing level and state flags.
var global environment.GlobalEnvironment
var eventingGlobal EventingGlobal

type EventingGlobal struct {
	BrokerClass        string `envconfig:"BROKER_CLASS" default:"MTChannelBasedBroker" required:"true"`
	BrokerTemplatesDir string `envconfig:"BROKER_TEMPLATES"`
}

func init() {
	// environment.InitFlags registers state and level filter flags.
	environment.InitFlags(flag.CommandLine)

	// Process EventingGlobal.
	if err := envconfig.Process("", &eventingGlobal); err != nil {
		log.Fatal("Failed to process env var", zap.Error(err))
	}
}

// TestMain is the first entry point for `go test`.
func TestMain(m *testing.M) {
	// We get a chance to parse flags to include the framework flags for the
	// framework as well as any additional flags included in the integration.
	flag.Parse()

	// EnableInjectionOrDie will enable client injection, this is used by the
	// testing framework for namespace management, and could be leveraged by
	// features to pull Kubernetes clients or the test environment out of the
	// context passed in the features.
	ctx, startInformers := injection.EnableInjectionOrDie(nil, nil) //nolint
	startInformers()

	// global is used to make instances of Environments, NewGlobalEnvironment
	// is passing and saving the client injection enabled context for use later.
	global = environment.NewGlobalEnvironment(ctx)

	// Run the tests.
	os.Exit(m.Run())
}
