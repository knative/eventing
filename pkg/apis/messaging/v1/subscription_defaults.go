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
)

func (s *Subscription) SetDefaults(ctx context.Context) {
	if s == nil {
		return
	}
	ctx = apis.WithinParent(ctx, s.ObjectMeta)
	s.Spec.SetDefaults(ctx)
}

func (ss *SubscriptionSpec) SetDefaults(ctx context.Context) {
	if ss == nil {
		return
	}

	ss.Subscriber.SetDefaults(ctx)
	ss.Reply.SetDefaults(ctx)
	ss.Delivery.SetDefaults(ctx)
}
