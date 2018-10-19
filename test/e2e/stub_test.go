// +build e2e

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

package e2e

import (
	"testing"

	"github.com/knative/eventing/test"
	"github.com/knative/pkg/test/logging"
)

const (
	serviceAccount = "e2e-sa"
)

func TestKnativeEventing(t *testing.T) {

	logger := logging.GetContextLogger("TestKnativeEventing")

	clients, cleaner := Setup(t, logger)

	test.CleanupOnInterrupt(func() { TearDown(clients, cleaner, logger) }, logger)
	defer TearDown(clients, cleaner, logger)

	logger.Infof("Creating ServiceAccount and Binding")

	err := CreateServiceAccountAndBinding(clients, serviceAccount, logger, cleaner)
	if err != nil {
		t.Fatalf("Failed to create ServiceAccount or Binding: %v", err)
	}

	// TODO test something
}
