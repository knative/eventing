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

package apiserver

import (
	"context"
	"encoding/json"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"k8s.io/client-go/rest"
	"knative.dev/eventing/pkg/adapter/v2"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/logging"
)

func init() {
	injection.Default.RegisterClient(withStoredHost)
}

// Key is used as the key for associating information
// with a context.Context.
type Key struct{}

func withStoredHost(ctx context.Context, cfg *rest.Config) context.Context {
	return context.WithValue(ctx, Key{}, cfg.Host)
}

// Get extracts the k8s Host from the context.
func Get(ctx context.Context) string {
	untyped := ctx.Value(Key{})
	if untyped == nil {
		logging.FromContext(ctx).Panic(
			"Unable to fetch k8s.io/client-go/rest/Config.Host from context.")
	}
	return untyped.(string)
}

// ---- New

func NewEnvConfig() adapter.EnvConfigAccessor {
	return &envConfig{}
}

func NewAdapter(ctx context.Context, processed adapter.EnvConfigAccessor, ceClient cloudevents.Client) adapter.Adapter {
	logger := logging.FromContext(ctx)
	env := processed.(*envConfig)

	config := Config{}
	if err := json.Unmarshal([]byte(env.ConfigJson), &config); err != nil {
		panic("failed to create config from json")
	}

	return &apiServerAdapter{
		discover: kubeclient.Get(ctx).Discovery(),
		k8s:      dynamicclient.Get(ctx),
		ce:       ceClient,
		source:   Get(ctx),
		name:     env.Name,
		config:   config,

		logger: logger,
	}
}
