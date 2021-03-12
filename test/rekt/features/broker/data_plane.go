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

package broker

import (
	"context"

	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/state"
)

func DataPlaneConformance(brokerName string) *feature.FeatureSet {
	fs := new(feature.FeatureSet)
	fs.Name = "Delivery Conformance"
	fs.Features = []feature.Feature{
		*DataPlaneIngress(brokerName),
		*DataPlaneDelivery(brokerName),
		*DataPlaneObservability(brokerName),
	}

	return fs
}

func DataPlaneIngress(brokerName string) *feature.Feature {
	f := new(feature.Feature)

	f.Setup("Set Broker Name", func(ctx context.Context, t feature.T) {
		state.SetOrFail(ctx, t, "brokerName", brokerName)
	})

	f.Stable("Delivery Conformance").
		Should("",
			skip)

	return f
}

func DataPlaneDelivery(brokerName string) *feature.Feature {
	f := new(feature.Feature)

	f.Setup("Set Broker Name", func(ctx context.Context, t feature.T) {
		state.SetOrFail(ctx, t, "brokerName", brokerName)
	})

	f.Stable("Delivery Conformance").
		Should("",
			skip)

	return f
}

func DataPlaneObservability(brokerName string) *feature.Feature {
	f := new(feature.Feature)

	f.Setup("Set Broker Name", func(ctx context.Context, t feature.T) {
		state.SetOrFail(ctx, t, "brokerName", brokerName)
	})

	f.Stable("Delivery Conformance").
		Should("",
			skip)

	return f
}

func skip(ctx context.Context, t feature.T) {
}
