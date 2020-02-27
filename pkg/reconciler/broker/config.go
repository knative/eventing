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
	"errors"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
)

type Config struct {
	DefaultChannelTemplate messagingv1beta1.ChannelTemplateSpec
}

func NewConfigFromConfigMapFunc(ctx context.Context) func(configMap *corev1.ConfigMap) (*Config, error) {
	return func(configMap *corev1.ConfigMap) (*Config, error) {

		apiVersion, ok := configMap.Data["channelTemplateSpec.apiVersion"]
		if !ok {
			return nil, errors.New("channelTemplateSpec.apiVersion not found in config")
		}

		kind, ok := configMap.Data["channelTemplateSpec.kind"]
		if !ok {
			return nil, errors.New("channelTemplateSpec.kind not found in config")
		}

		// Spec is optional.
		var spec *runtime.RawExtension
		if rs, ok := configMap.Data["channelTemplateSpec.specJson"]; ok {
			spec = &runtime.RawExtension{Raw: []byte(strings.TrimSpace(rs))}
		}

		return &Config{
			DefaultChannelTemplate: messagingv1beta1.ChannelTemplateSpec{
				TypeMeta: metav1.TypeMeta{
					APIVersion: apiVersion,
					Kind:       kind,
				},
				Spec: spec,
			},
		}, nil
	}
}
