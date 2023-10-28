//go:build e2e
// +build e2e

/*
Copyright 2023 The Knative Authors

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

package auth

import (
	"os"
	"testing"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "knative.dev/pkg/system/testing"

	"knative.dev/reconciler-test/pkg/environment"
)

// global is the singleton instance of GlobalEnvironment. It is used to parse
// the testing config for the test run. The config will specify the cluster
// config as well as the parsing level and state flags.
var global environment.GlobalEnvironment

// TestMain is the first entry point for `go test`.
func TestMain(m *testing.M) {
	// global is used to make instances of Environments, NewStandardGlobalEnvironment
	// is initializing the client injection enabled context for use later.
	global = environment.NewStandardGlobalEnvironment()

	// Run the tests.
	os.Exit(m.Run())
}
