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
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/sidecar/configmap"
	"github.com/knative/eventing/pkg/sidecar/fanout"
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
	"github.com/knative/eventing/pkg/utils"
	"go.uber.org/zap"
	yaml "gopkg.in/yaml.v2"
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
						- subscriberURI: event-changer.default.svc.` + utils.GetClusterDomainName() + `
						  replyURI: message-dumper-bar.default.svc.` + utils.GetClusterDomainName() + `
						- subscriberURI: message-dumper-foo.default.svc.` + utils.GetClusterDomainName() + `
						- replyURI: message-dumper-bar.default.svc.` + utils.GetClusterDomainName() + `
				  - namespace: default
					name: c2
					fanoutConfig:
					  subscriptions:
						- replyURI: message-dumper-foo.default.svc.` + utils.GetClusterDomainName() + `
				  - namespace: other
					name: c3
					fanoutConfig:
					  subscriptions:
						- replyURI: message-dumper-foo.default.svc.` + utils.GetClusterDomainName(),
			expected: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						Namespace: "default",
						Name:      "c1",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.ChannelSubscriberSpec{
								{
									SubscriberURI: "event-changer.default.svc." + utils.GetClusterDomainName(),
									ReplyURI:      "message-dumper-bar.default.svc." + utils.GetClusterDomainName(),
								},
								{
									SubscriberURI: "message-dumper-foo.default.svc." + utils.GetClusterDomainName(),
								},
								{
									ReplyURI: "message-dumper-bar.default.svc." + utils.GetClusterDomainName(),
								},
							},
						},
					},
					{
						Namespace: "default",
						Name:      "c2",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.ChannelSubscriberSpec{
								{
									ReplyURI: "message-dumper-foo.default.svc." + utils.GetClusterDomainName(),
								},
							},
						},
					},
					{
						Namespace: "other",
						Name:      "c3",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.ChannelSubscriberSpec{
								{
									ReplyURI: "message-dumper-foo.default.svc." + utils.GetClusterDomainName(),
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
				var cleanup func()
				dir, cleanup = createTempDir(t)
				defer cleanup()
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
							Subscriptions: []eventingduck.ChannelSubscriberSpec{
								{
									ReplyURI: "foo.bar",
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
							Subscriptions: []eventingduck.ChannelSubscriberSpec{
								{
									ReplyURI: "foo.bar",
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
							Subscriptions: []eventingduck.ChannelSubscriberSpec{
								{
									ReplyURI: "foo.bar",
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
							Subscriptions: []eventingduck.ChannelSubscriberSpec{
								{
									SubscriberURI: "baz.qux",
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
			dir, cleanup := createTempDir(t)
			defer cleanup()
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
			go func() {
				_ = cmw.Start(stopCh)
			}()
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

func createTempDir(t *testing.T) (string, func()) {
	dir, err := ioutil.TempDir("", "configMapHandlerTest")
	if err != nil {
		t.Errorf("Unable to make temp directory: %v", err)
	}
	return dir, func() {
		_ = os.RemoveAll(dir)
	}
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
		err := atomicWriteFile(t, fmt.Sprintf("%s/%s", dir, configmap.MultiChannelFanoutConfigKey), []byte(leftSpaceConfig), 0700)
		if err != nil {
			t.Errorf("Problem writing the config file: %v", err)
		}
	}
}

func atomicWriteFile(t *testing.T, file string, bytes []byte, perm os.FileMode) error {
	// In order to more closely replicate how K8s writes ConfigMaps to the file system, we will
	// atomically swap out the file by writing it to a temp directory, then renaming it into the
	// directory we are watching.
	tempDir, cleanup := createTempDir(t)
	defer cleanup()

	tempFile := fmt.Sprintf("%s/%s", tempDir, "temp")
	err := ioutil.WriteFile(tempFile, bytes, perm)
	if err != nil {
		return err
	}
	return os.Rename(tempFile, file)
}
