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
	"knative.dev/reconciler-test/pkg/feature"
)

func DataPlaneConformance(channelName string) *feature.FeatureSet {
	fs := &feature.FeatureSet{
		Name: "Knative Channel Specification - Data Plane",
		Features: []feature.Feature{
			*DataPlaneChannel(channelName),
		},
	}

	return fs
}

func DataPlaneChannel(channelName string) *feature.Feature {
	f := feature.NewFeatureNamed("Conformance")

	f.Setup("Set Channel Name", setChannelableName(channelName))

	f.Stable("Channel Subscriber Status").
		Must("The ready field of the subscriber identified by its uid MUST be set to True when the subscription is ready to be processed",
			todo)

	return f
}
