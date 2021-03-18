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

func (s *Subscription) SetDefaults(ctx context.Context) {
	s.Spec.SetDefaults(ctx)
}

func (ss *SubscriptionSpec) SetDefaults(ctx context.Context) {
	// HACK if a channel ref is a kafka channel ref, we need to hack it around to use only v1beta1
	//  TODO(slinkydeveloper) REMOVE AFTER 0.22 release
	if ss.Channel.Kind == "KafkaChannel" && ss.Channel.APIVersion == "messaging.knative.dev/v1alpha1" {
		ss.Channel.APIVersion = "messaging.knative.dev/v1beta1"
	}
}
