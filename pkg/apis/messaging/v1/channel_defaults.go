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

package v1

import (
	"context"

	"knative.dev/pkg/apis"

	"knative.dev/eventing/pkg/apis/messaging"
	"knative.dev/eventing/pkg/apis/messaging/config"
)

func (c *Channel) SetDefaults(ctx context.Context) {
	if c.Annotations == nil {
		c.Annotations = make(map[string]string)
	}
	if _, ok := c.Annotations[messaging.SubscribableDuckVersionAnnotation]; !ok {
		c.Annotations[messaging.SubscribableDuckVersionAnnotation] = "v1"
	}

	c.Spec.SetDefaults(apis.WithinParent(ctx, c.ObjectMeta))
}

func (cs *ChannelSpec) SetDefaults(ctx context.Context) {
	if cs.ChannelTemplate != nil {
		return
	}

	cfg := config.FromContextOrDefaults(ctx)
	c, err := cfg.ChannelDefaults.GetChannelConfig(apis.ParentMeta(ctx).Namespace)
	if err == nil {
		cs.ChannelTemplate = &ChannelTemplateSpec{
			c.TypeMeta,
			c.Spec,
		}
	}
	cs.Delivery.SetDefaults(ctx)
}

// ChannelDefaulter sets the default Channel CRD and Arguments on Channels that do not
// specify any implementation.
type ChannelDefaulter interface {
	// GetDefault determines the default Channel CRD for the given namespace.
	GetDefault(namespace string) *ChannelTemplateSpec
}
