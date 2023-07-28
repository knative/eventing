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

package multichannelfanout

import (
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/channel/fanout"
	"knative.dev/eventing/pkg/eventtype"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

type Config struct {
	ChannelConfigs []ChannelConfig
}

// ChannelConfig is the configuration for a single Channel.
type ChannelConfig struct {
	Namespace          string
	Name               string
	HostName           string
	Path               string
	FanoutConfig       fanout.Config
	EventTypeHandler   *eventtype.EventTypeAutoHandler
	ChannelAddressable *duckv1.KReference
	ChannelUID         *types.UID
}
