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

	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/apis"
)

func (s *Sequence) SetDefaults(ctx context.Context) {
	withNS := apis.WithinParent(ctx, s.ObjectMeta)
	if s != nil && s.Spec.ChannelTemplate == nil {
		// The singleton may not have been set, if so ignore it and validation will reject the
		// Channel.
		if cd := eventingduckv1alpha1.ChannelDefaulterSingleton; cd != nil {
			channelTemplate := cd.GetDefault(s.Namespace)
			s.Spec.ChannelTemplate = channelTemplate
		}
	}
	s.Spec.SetDefaults(withNS)
}

func (ss *SequenceSpec) SetDefaults(ctx context.Context) {
	// Default the namespace for all the steps.
	for _, s := range ss.Steps {
		s.SetDefaults(ctx)
	}
	// Default the reply
	if ss.Reply != nil {
		ss.Reply.SetDefaults(ctx)
	}
}
