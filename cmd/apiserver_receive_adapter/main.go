/*
Copyright 2019 The Knative Authors.

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

package main

import (
	filteredFactory "knative.dev/pkg/client/injection/kube/informers/factory/filtered"
	"knative.dev/pkg/signals"

	"knative.dev/eventing/pkg/adapter/apiserver"
	"knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/eventing/pkg/eventingtls"
)

const (
	component = "apiserversource"
)

func main() {
	ctx := signals.NewContext()
	ctx = adapter.WithInjectorEnabled(ctx)

	ctx = filteredFactory.WithSelectors(ctx,
		eventingtls.TrustBundleLabelSelector,
	)

	adapter.MainWithContext(ctx, component, apiserver.NewEnvConfig, apiserver.NewAdapter)
}
