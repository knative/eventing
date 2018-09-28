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

package parse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	// The config key that contains all the configuration data.
	MultiChannelFanoutConfigKey = "multiChannelFanoutConfig"
)

// ConfigMapData attempts to parse the config map's data into a multichannelfanout.Config.
func ConfigMapData(logger *zap.Logger, data map[string]string) (*multichannelfanout.Config, error) {
	if _, present := data[MultiChannelFanoutConfigKey]; !present {
		logger.Error("Expected key not found", zap.String("key", MultiChannelFanoutConfigKey))
		return nil, fmt.Errorf("expected key not found: %v", MultiChannelFanoutConfigKey)
	}
	jb, err := yaml.ToJSON([]byte(data[MultiChannelFanoutConfigKey]))
	if err != nil {
		logger.Error("Unable to convert multiChannelFanoutConfig to JSON", zap.Error(err))
		return nil, err
	}
	var conf multichannelfanout.Config
	err = unmarshallJsonDisallowUnknownFields(jb, &conf)
	return &conf, err
}

// unmarshallJsonDisallowUnknownFields unmarshalls JSON, but unlike json.Unmarshall, will fail if
// given an unknown field (rather than json.Unmarshall's ignoring the unknown field).
func unmarshallJsonDisallowUnknownFields(jb []byte, v interface{}) error {
	d := json.NewDecoder(bytes.NewReader(jb))
	d.DisallowUnknownFields()
	return d.Decode(v)
}
