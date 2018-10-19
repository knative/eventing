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

package configmap

import (
	"encoding/json"
	"fmt"
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
	"go.uber.org/zap"
)

const (
	// MultiChannelFanoutConfigKey is the key in the ConfigMap that contains all the configuration
	// data.
	MultiChannelFanoutConfigKey = "multiChannelFanoutConfig"
)

// ConfigMapData attempts to parse the config map's data into a multichannelfanout.Config.
// orig == NewFanoutConfig(SerializeConfig(orig))
func NewFanoutConfig(logger *zap.Logger, data map[string]string) (*multichannelfanout.Config, error) {
	str, present := data[MultiChannelFanoutConfigKey]
	if !present {
		logger.Error("Expected key not found", zap.String("key", MultiChannelFanoutConfigKey))
		return nil, fmt.Errorf("expected key not found: %v", MultiChannelFanoutConfigKey)
	}
	return multichannelfanout.Parse(logger, str)
}

// SerializeConfig takes in a multichannelfanout.Config and generates the ConfigMap equivalent.
// orig == NewFanoutConfig(SerializeConfig(orig))
func SerializeConfig(config multichannelfanout.Config) (map[string]string, error) {
	jb, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		MultiChannelFanoutConfigKey: string(jb),
	}, nil
}
