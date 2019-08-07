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
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/provisioners/fanout"
)

// Config for a multichannelfanout.Handler.
type Config struct {
	// The configuration of each channel in this handler.
	ChannelConfigs []ChannelConfig `json:"channelConfigs"`
}

// ChannelConfig is the configuration for a single Channel.
type ChannelConfig struct {
	Namespace    string        `json:"namespace"`
	Name         string        `json:"name"`
	HostName     string        `json:"hostname"`
	FanoutConfig fanout.Config `json:"fanoutConfig"`
}

// NewConfigFromChannels creates a new Config from the list of channels.
func NewConfigFromChannels(channels []v1alpha1.Channel) *Config {
	cc := make([]ChannelConfig, 0)
	for _, c := range channels {
		channelConfig := ChannelConfig{
			Namespace: c.Namespace,
			Name:      c.Name,
			HostName:  c.Status.Address.GetURL().Host,
		}
		if c.Spec.Subscribable != nil {
			channelConfig.FanoutConfig = fanout.Config{
				AsyncHandler:  true,
				Subscriptions: c.Spec.Subscribable.Subscribers,
			}
		}
		cc = append(cc, channelConfig)
	}
	return &Config{
		ChannelConfigs: cc,
	}
}
