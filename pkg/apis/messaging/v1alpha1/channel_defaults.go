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
	v1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	"knative.dev/pkg/apis"
)

func (c *Channel) SetDefaults(ctx context.Context) {
	if c != nil && c.Spec.ChannelTemplate == nil {
		cfg := config.FromContextOrDefaults(ctx)
		defaultChannel, err := cfg.ChannelDefaults.GetChannelConfig(apis.ParentMeta(ctx).Namespace)
		if err == nil {
			c.Spec.ChannelTemplate = &v1beta1.ChannelTemplateSpec{
				TypeMeta: defaultChannel.TypeMeta,
				Spec:     defaultChannel.Spec,
			}
		}
	}
	c.Spec.SetDefaults(ctx)
}

func (cs *ChannelSpec) SetDefaults(ctx context.Context) {}
