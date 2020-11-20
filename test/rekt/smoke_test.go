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
	"testing"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	_ "knative.dev/pkg/system/testing"

	"knative.dev/eventing/test/rekt/features/broker"
)

// TestSmoke_Broker
func TestSmoke_Broker(t *testing.T) {
	t.Parallel()

	ctx, env := global.Environment()

	names := []string{
		"default",
		"customname",
		"name-with-dash",
		"name1with2numbers3",
		"name63-01234567890123456789012345678901234567890123456789012345",
	}

	for _, name := range names {
		env.Test(ctx, t, broker.BrokerGoesReady(name))
	}

	env.Finish()
}
