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

// TODO This should be placed in channel_defaults.go within messaging once Subscription is moved to messaging.
//  Context: there is a cyclic dependency between eventing and messaging if we place this in messaging. Broker needs to
//  depend on this, which is fine. But the problem arises due to messaging depending on eventing, mainly on
//  Subscription-related objects for the Sequence type. We should first move Subscription down to messaging and then we
//  can move this down. See https://github.com/knative/eventing/issues/1562.
//

// ChannelDefaulter sets the default Channel CRD and Arguments on Channels that do not
// specify any implementation.
type ChannelDefaulter interface {
	// GetDefault determines the default Channel CRD for the given namespace.
	GetDefault(namespace string) *ChannelTemplateSpec
}

var (
	// ChannelDefaulterSingleton is the global singleton used to default Channels that do not
	// specify a Channel CRD.
	ChannelDefaulterSingleton ChannelDefaulter
)
