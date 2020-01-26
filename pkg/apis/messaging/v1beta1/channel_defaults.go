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
)

var (
	// ChannelDefaulterSingleton is the global singleton used to default Channels that do not
	// specify a Channel CRD.
	ChannelDefaulterSingleton ChannelDefaulter
)

func (c *Channel) SetDefaults(ctx context.Context) {
	if c != nil && c.Spec.ChannelTemplate == nil {
		// The singleton may not have been set, if so ignore it and validation will reject the
		// Channel.
		if cd := ChannelDefaulterSingleton; cd != nil {
			c.Spec.ChannelTemplate = cd.GetDefault(c.Namespace)
		}
	}
	c.Spec.SetDefaults(ctx)
}

func (cs *ChannelSpec) SetDefaults(ctx context.Context) {}

// ChannelDefaulter sets the default Channel CRD and Arguments on Channels that do not
// specify any implementation.
type ChannelDefaulter interface {
	// GetDefault determines the default Channel CRD for the given namespace.
	GetDefault(namespace string) *ChannelTemplateSpec
}
