/*
Copyright 2021 The Knative Authors

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

package channel

import (
	"fmt"

	"knative.dev/eventing/test/rekt/resources/channel"
	"knative.dev/eventing/test/rekt/resources/channel_impl"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
)

// GoesReady returns a feature testing if a Channel becomes ready.
func GoesReady(name string, cfg ...manifest.CfgFn) *feature.Feature {
	f := feature.NewFeatureNamed("Channel goes ready.")

	f.Setup(fmt.Sprintf("install a Channel named %q", name), channel.Install(name, cfg...))

	f.Stable("Channel").
		Must("be ready", channel.IsReady(name)).
		Must("be addressable", channel.IsAddressable(name))

	return f
}

// ImplGoesReady returns a feature testing if a channel impl becomes ready.
func ImplGoesReady(name string, cfg ...manifest.CfgFn) *feature.Feature {
	f := feature.NewFeatureNamed("ChannelImpl goes ready.")

	f.Setup(fmt.Sprintf("install a %s named %q", channel_impl.GVK().Kind, name), channel_impl.Install(name, cfg...))

	f.Stable("ChannelImpl").
		Must("be ready", channel_impl.IsReady(name)).
		Must("be addressable", channel_impl.IsAddressable(name))

	return f
}
