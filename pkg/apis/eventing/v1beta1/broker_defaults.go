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

package v1beta1

import (
	"context"

	"knative.dev/eventing/pkg/apis/config"
	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/pkg/apis"
)

func (b *Broker) SetDefaults(ctx context.Context) {
	// Default Spec fields.
	withNS := apis.WithinParent(ctx, b.ObjectMeta)
	b.Spec.SetDefaults(withNS)

	// Check the annotation and default if necessary
	annotations := b.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string, 1)
	}
	if _, present := annotations[eventing.BrokerClassKey]; !present {
		cfg := config.FromContextOrDefaults(withNS)
		c, err := cfg.Defaults.GetBrokerClass(b.Namespace)
		if err == nil {
			annotations[eventing.BrokerClassKey] = c
			b.SetAnnotations(annotations)
		}
	}
}

func (bs *BrokerSpec) SetDefaults(ctx context.Context) {
	if bs.Config != nil {
		return
	}

	cfg := config.FromContextOrDefaults(ctx)
	c, err := cfg.Defaults.GetBrokerConfig(apis.ParentMeta(ctx).Namespace)
	if err == nil {
		bs.Config = c
	}
}
