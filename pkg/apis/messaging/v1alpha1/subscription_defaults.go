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

	"knative.dev/pkg/apis"
)

func (s *Subscription) SetDefaults(ctx context.Context) {
	withNS := apis.WithinParent(ctx, s.ObjectMeta)
	s.Spec.SetDefaults(withNS)
}

func (ss *SubscriptionSpec) SetDefaults(ctx context.Context) {
	// Default NS to Subscriber and Reply
	if ss.Subscriber != nil {
		ss.Subscriber.SetDefaults(ctx)
	}
	if ss.Reply != nil {
		ss.Reply.SetDefaults(ctx)
	}
}
