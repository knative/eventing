/*
Copyright 2019 The Knative Authors

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

package apiserversource

import (
	"context"
	"github.com/knative/pkg/injection"
	"github.com/knative/pkg/logging"
	"k8s.io/client-go/rest"
)

func init() {
	// This is a hack to gain access to the rest config.
	injection.Default.RegisterClient(withCfgHost)
}

// HostKey is used as the key for associating information
// with a context.Context.
type HostKey struct{}

func withCfgHost(ctx context.Context, cfg *rest.Config) context.Context {
	return context.WithValue(ctx, HostKey{}, cfg.Host)
}

// GetCfgHost extracts the k8s rest config host from the context.
func GetCfgHost(ctx context.Context) string {
	untyped := ctx.Value(HostKey{})
	if untyped == nil {
		logging.FromContext(ctx).Fatal("Failed to load cfg host from context.")
	}
	return untyped.(string)
}
