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

package v1alpha1

import (
	"context"

	"knative.dev/eventing/pkg/apis/messaging/config"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	"knative.dev/pkg/apis"
)

func (b *Broker) SetDefaults(ctx context.Context) {
	withNS := apis.WithinParent(ctx, b.ObjectMeta)
	b.Spec.SetDefaults(withNS)
}

func (bs *BrokerSpec) SetDefaults(ctx context.Context) {
	if bs.Config == nil {
		// If we haven't configured the new channelTemplate,
		// then set the default channel to the new channelTemplate.
		if bs.ChannelTemplate == nil {
			cfg := config.FromContextOrDefaults(ctx)
			c, err := cfg.ChannelDefaults.GetChannelConfig(apis.ParentMeta(ctx).Namespace)

			if err == nil {
				bs.ChannelTemplate = &messagingv1beta1.ChannelTemplateSpec{
					c.TypeMeta,
					c.Spec,
				}
			}
		}
	} else {
		if bs.Config.Namespace == "" {
			bs.Config.Namespace = apis.ParentMeta(ctx).Namespace
		}
	}
}
