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

package filesystem

import (
	"errors"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/knative/eventing/pkg/sidecar/configmap"
	"github.com/knative/eventing/pkg/sidecar/fanout"
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestReadConfigMap(t *testing.T) {
	testCases := []struct {
		name        string
		createDir   bool
		config      string
		expected    *multichannelfanout.Config
		expectedErr bool
	}{
		{
			name:      "dir does not exist",
			createDir: false,
		},
		{
			name:        "no data",
			createDir:   true,
			expectedErr: true,
		},
		{
			name:      "invalid YAML",
			createDir: true,
			config: `
				key:
				  - value
				 - different indent level
				`,
			expectedErr: true,
		},
		{
			name:        "valid YAML -- invalid JSON",
			config:      "{ nil: Key }",
			createDir:   true,
			expectedErr: true,
		},
		{
			name:        "unknown field",
			config:      "{ channelConfigs: [ { not: a-defined-field } ] }",
			createDir:   true,
			expectedErr: true,
		},
		{
			name:      "valid",
			createDir: true,
			config: `
				channelConfigs:
				  - namespace: default
					name: c1
					fanoutConfig:
					  subscriptions:
						- callableDomain: event-changer.default.svc.cluster.local
						  sinkableDomain: message-dumper-bar.default.svc.cluster.local
						- callableDomain: message-dumper-foo.default.svc.cluster.local
						- sinkableDomain: message-dumper-bar.default.svc.cluster.local
				  - namespace: default
					name: c2
					fanoutConfig:
					  subscriptions:
						- sinkableDomain: message-dumper-foo.default.svc.cluster.local
				  - namespace: other
					name: c3
					fanoutConfig:
					  subscriptions:
						- sinkableDomain: message-dumper-foo.default.svc.cluster.local
				`,
			expected: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						Namespace: "default",
						Name:      "c1",
						FanoutConfig: fanout.Config{
							Subscriptions: []duckv1alpha1.ChannelSubscriberSpec{
								{
									CallableDomain: "event-changer.default.svc.cluster.local",
									SinkableDomain: "message-dumper-bar.default.svc.cluster.local",
								},
								{
									CallableDomain: "message-dumper-foo.default.svc.cluster.local",
								},
								{
									SinkableDomain: "message-dumper-bar.default.svc.cluster.local",
								},
							},
						},
					},
					{
						Namespace: "default",
						Name:      "c2",
						FanoutConfig: fanout.Config{
							Subscriptions: []duckv1alpha1.ChannelSubscriberSpec{
								{
									SinkableDomain: "message-dumper-foo.default.svc.cluster.local",
								},
							},
						},
					},
					{
						Namespace: "other",
						Name:      "c3",
						FanoutConfig: fanout.Config{
							Subscriptions: []duckv1alpha1.ChannelSubscriberSpec{
								{
									SinkableDomain: "message-dumper-foo.default.svc.cluster.local",
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var dir string
			if tc.createDir {
				dir = createTempDir(t)
				defer os.RemoveAll(dir)
			} else {
				dir = "/tmp/doesNotExist"
			}
			writeConfigString(t, dir, tc.config)
			c, e := readConfigMap(zap.NewNop(), dir)
			if tc.expectedErr {
				if e == nil {
					t.Errorf("Expected an error, actual nil")
				}
				return
			}
			if !cmp.Equal(c, tc.expected) {
				t.Errorf("Unexpected config. Expected '%v'. Actual '%v'.", tc.expected, c)
			}
		})
	}
}

func TestWatch(t *testing.T) {
	testCases := map[string]struct {
		initialConfigErr error
		initialConfig    *multichannelfanout.Config
		updateConfigErr  error
		updateConfig     *multichannelfanout.Config
	}{
		"error applying initial config": {
			initialConfig:    &multichannelfanout.Config{},
			initialConfigErr: errors.New("test-induced error"),
		},
		"read initial config": {
			initialConfig: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						Namespace: "default",
						Name:      "c1",
						FanoutConfig: fanout.Config{
							Subscriptions: []duckv1alpha1.ChannelSubscriberSpec{
								{
									SinkableDomain: "foo.bar",
								},
							},
						},
					},
				},
			},
		},
		"error apply updated config": {
			initialConfig: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						Namespace: "default",
						Name:      "c1",
						FanoutConfig: fanout.Config{
							Subscriptions: []duckv1alpha1.ChannelSubscriberSpec{
								{
									SinkableDomain: "foo.bar",
								},
							},
						},
					},
				},
			},
			updateConfigErr: errors.New("test-induced error"),
		},
		"update config": {
			initialConfig: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						Namespace: "default",
						Name:      "c1",
						FanoutConfig: fanout.Config{
							Subscriptions: []duckv1alpha1.ChannelSubscriberSpec{
								{
									SinkableDomain: "foo.bar",
								},
							},
						},
					},
				},
			},
			updateConfig: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						Namespace: "default",
						Name:      "new-channel",
						FanoutConfig: fanout.Config{
							Subscriptions: []duckv1alpha1.ChannelSubscriberSpec{
								{
									CallableDomain: "baz.qux",
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
			dir := createTempDir(t)
			defer os.RemoveAll(dir)
			writeConfig(t, dir, tc.initialConfig)

			cuc := &configUpdatedChecker{
				updateConfigErr: tc.initialConfigErr,
			}
			cmw, err := NewConfigMapWatcher(zap.NewNop(), dir, cuc.updateConfig)
			if err != nil {
				if tc.initialConfigErr != err {
					t.Errorf("Unexpected error making ConfigMapWatcher. Expected: '%v'. Actual '%v'", tc.initialConfigErr, err)
				}
				return
			}
			ac := cuc.getConfig()
			if !cmp.Equal(tc.initialConfig, ac) {
				t.Errorf("Unexpected initial config. Expected '%v'. Actual '%v'", tc.initialConfig, ac)
			}

			stopCh := make(chan struct{})
			go cmw.Start(stopCh)
			defer func() {
				close(stopCh)
			}()
			// Sadly, the test is flaky unless we sleep here, waiting for the file system
			// watcher to truly start.
			time.Sleep(100 * time.Millisecond)

			if tc.updateConfigErr != nil {
				cuc.updateConfigErr = tc.updateConfigErr
			}

			expected := tc.initialConfig
			if tc.updateConfig != nil {
				expected = tc.updateConfig
			}

			cuc.updateCalled = make(chan struct{}, 1)
			writeConfig(t, dir, expected)
			// The watcher is running in another goroutine, give it some time to notice the
			// change.
			select {
			case <-cuc.updateCalled:
				break
			case <-time.After(5 * time.Second):
				t.Errorf("Time out waiting for watcher to notice change.")
			}

			ac = cuc.getConfig()
			if !cmp.Equal(ac, expected) {
				t.Errorf("Unexpected update config. Expected '%v'. Actual '%v'", expected, ac)
			}
		})
	}
}

type configUpdatedChecker struct {
	configLock      sync.Mutex
	config          *multichannelfanout.Config
	updateCalled    chan struct{}
	updateConfigErr error
}

func (cuc *configUpdatedChecker) updateConfig(config *multichannelfanout.Config) error {
	cuc.configLock.Lock()
	defer cuc.configLock.Unlock()
	cuc.config = config
	if cuc.updateCalled != nil {
		cuc.updateCalled <- struct{}{}
	}
	return cuc.updateConfigErr
}

func (cuc *configUpdatedChecker) getConfig() *multichannelfanout.Config {
	cuc.configLock.Lock()
	defer cuc.configLock.Unlock()
	return cuc.config
}

func createTempDir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "configMapHandlerTest")
	if err != nil {
		t.Errorf("Unable to make temp directory: %v", err)
	}
	return dir
}

func writeConfig(t *testing.T, dir string, config *multichannelfanout.Config) {
	if config != nil {
		yb, err := yaml.Marshal(config)
		if err != nil {
			t.Errorf("Unable to marshal the config")
		}
		writeConfigString(t, dir, string(yb))
	}
}

func writeConfigString(t *testing.T, dir, config string) {
	if config != "" {
		// Golang editors tend to replace leading spaces with tabs. YAML is left whitespace
		// sensitive, so let's replace the tabs with spaces.
		leftSpaceConfig := strings.Replace(config, "\t", "    ", -1)
		err := ioutil.WriteFile(fmt.Sprintf("%s/%s", dir, configmap.MultiChannelFanoutConfigKey), []byte(leftSpaceConfig), 0700)
		if err != nil {
			t.Errorf("Problem writing the config file: %v", err)
		}
	}
}
