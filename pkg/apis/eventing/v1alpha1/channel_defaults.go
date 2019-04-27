/*
Copyright 2018 The Knative Authors

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
	"github.com/knative/eventing/pkg/apis/eventing"
	"github.com/knative/pkg/apis"
	"k8s.io/apimachinery/pkg/api/equality"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ChannelProvisionerDefaulter sets the default Provisioner and Arguments on Channels that do not
// specify any Provisioner.
type ChannelProvisionerDefaulter interface {
	// GetDefault determines the default provisioner and arguments for the given channel. It does
	// not modify the given channel. It may return nil for either or both.
	GetDefault(c *Channel) (*corev1.ObjectReference, *runtime.RawExtension)
}

var (
	// ChannelDefaulterSingleton is the global singleton used to default Channels that do not
	// specify any provisioner.
	ChannelDefaulterSingleton ChannelProvisionerDefaulter
)

func (c *Channel) SetDefaults(ctx context.Context) {
	if c != nil && c.Spec.Provisioner == nil {
		// The singleton may not have been set, if so ignore it and validation will reject the
		// Channel.
		if cd := ChannelDefaulterSingleton; cd != nil {
			prov, args := cd.GetDefault(c.DeepCopy())
			c.Spec.Provisioner = prov
			c.Spec.Arguments = args
		}
	}
	c.Spec.SetDefaults(ctx)

	if ui := apis.GetUserInfo(ctx); ui != nil {
		ans := c.GetAnnotations()
		if ans == nil {
			ans = map[string]string{}
			defer c.SetAnnotations(ans)
		}

		if apis.IsInUpdate(ctx) {
			old := apis.GetBaseline(ctx).(*Channel)
			if equality.Semantic.DeepEqual(old.Spec, c.Spec) {
				return
			}
			ans[eventing.UpdaterAnnotation] = ui.Username
		} else {
			ans[eventing.CreatorAnnotation] = ui.Username
			ans[eventing.UpdaterAnnotation] = ui.Username
		}
	}
}

func (cs *ChannelSpec) SetDefaults(ctx context.Context) {}
