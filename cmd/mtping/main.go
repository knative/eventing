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

package main

import (
	"knative.dev/pkg/signals"

	"knative.dev/eventing/pkg/adapter/mtping"
	"knative.dev/eventing/pkg/adapter/v2"
)

const (
	component = "pingsource-mt-adapter"
)

func main() {
	ctx := signals.NewContext()
	ctx = adapter.WithConfigMapWatcherEnabled(ctx)
	ctx = adapter.WithInjectorEnabled(ctx)
	adapter.MainWithContext(ctx, component, mtping.NewEnvConfig, mtping.NewAdapter)
}
