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

package watcher

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	sidecarconfigmap "github.com/knative/eventing/pkg/sidecar/configmap"
	"github.com/knative/eventing/pkg/sidecar/fanout"
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
	"github.com/knative/pkg/configmap"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	namespace = "test-namespace"
	name      = "test-name"
)

func TestReconcile(t *testing.T) {
	testCases := map[string]struct {
		config          map[string]string
		updateConfigErr error
		expectedConfig  *multichannelfanout.Config
	}{
		"missing key": {
			config:         map[string]string{},
			expectedConfig: nil,
		},
		"cannot parse cm": {
			config: map[string]string{
				sidecarconfigmap.MultiChannelFanoutConfigKey: "invalid config",
			},
			expectedConfig: nil,
		},
		"configUpdated fails": {
			config: map[string]string{
				sidecarconfigmap.MultiChannelFanoutConfigKey: "",
			},
			updateConfigErr: errors.New("test-error"),
			expectedConfig:  &multichannelfanout.Config{},
		},
		"success": {
			config: map[string]string{
				sidecarconfigmap.MultiChannelFanoutConfigKey: `
                    channelConfigs:
                        - name: foo
                          namespace: bar
                          fanoutConfig:
                              subscriptions:
                                  - callableURI: callable
                                    sinkableURI: sinkable`,
			},
			expectedConfig: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						Name:      "foo",
						Namespace: "bar",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.ChannelSubscriberSpec{
								{
									CallableURI: "callable",
									SinkableURI: "sinkable",
								},
							},
						},
					},
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			cuc := &configUpdatedChecker{
				updateConfigErr: tc.updateConfigErr,
			}

			r, err := NewWatcher(zap.NewNop(), nil, namespace, name, cuc.updateConfig)
			if err != nil {
				t.Errorf("Error creating watcher: %v", err)
			}
			iw := r.(*configmap.InformedWatcher)
			iw.OnChange(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				},
				Data: tc.config,
			})

			if diff := cmp.Diff(tc.expectedConfig, cuc.config); diff != "" {
				t.Errorf("Unexpected config (-want +got): %v", diff)
			}
		})
	}
}

type configUpdatedChecker struct {
	config          *multichannelfanout.Config
	updateConfigErr error
}

func (cuc *configUpdatedChecker) updateConfig(config *multichannelfanout.Config) error {
	cuc.config = config
	return cuc.updateConfigErr
}
