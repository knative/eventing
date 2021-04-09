/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"go.uber.org/zap"
	"sigs.k8s.io/yaml"

	corev1 "k8s.io/api/core/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/pkg/logging"
)

type Config struct {
	DefaultChannelTemplate messagingv1.ChannelTemplateSpec
}

const (
	channelTemplateSpec = "channelTemplateSpec"
)

func NewConfigFromConfigMapFunc(ctx context.Context) func(configMap *corev1.ConfigMap) (*Config, error) {
	return func(configMap *corev1.ConfigMap) (*Config, error) {
		config := &Config{
			DefaultChannelTemplate: messagingv1.ChannelTemplateSpec{},
		}

		temp, present := configMap.Data[channelTemplateSpec]
		if !present {
			logging.FromContext(ctx).Infow("ConfigMap is missing key", zap.String("key", channelTemplateSpec), zap.Any("configMap", configMap))
			return nil, errors.New("not found")
		}

		if temp == "" {
			logging.FromContext(ctx).Infow("ConfigMap's value was the empty string, ignoring it.", zap.Any("configMap", configMap))
			return nil, errors.New("empty value for config")
		}

		j, err := yaml.YAMLToJSON([]byte(temp))
		if err != nil {
			return nil, fmt.Errorf("ConfigMap's value could not be converted to JSON. %w, %s", err, temp)
		}

		if err := json.Unmarshal(j, &config.DefaultChannelTemplate); err != nil {
			return nil, fmt.Errorf("ConfigMap's value could not be unmarshaled. %w, %s", err, string(j))
		}

		return config, nil
	}
}
