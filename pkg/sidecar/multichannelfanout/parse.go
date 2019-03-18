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

package multichannelfanout

import (
	"bytes"
	"encoding/json"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// Parse attempts to parse the YAML string into a multichannelfanout.Config.
func Parse(logger *zap.Logger, str string) (*Config, error) {
	jb, err := yaml.ToJSON([]byte(str))
	if err != nil {
		logger.Error("Unable to convert str to JSON", zap.Error(err), zap.String("str", str))
		return nil, err
	}
	var conf Config
	err = unmarshalJsonDisallowUnknownFields(jb, &conf)
	return &conf, err
}

// unmarshalJsonDisallowUnknownFields unmarshalls JSON, but unlike json.Unmarshal, will fail if
// given an unknown field (rather than json.Unmarshall's ignoring the unknown field).
func unmarshalJsonDisallowUnknownFields(jb []byte, v interface{}) error {
	d := json.NewDecoder(bytes.NewReader(jb))
	d.DisallowUnknownFields()
	return d.Decode(v)
}
