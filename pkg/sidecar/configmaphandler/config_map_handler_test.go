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

package configmaphandler

import (
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/knative/eventing/pkg/sidecar/clientfactory/fake"
	"github.com/knative/eventing/pkg/sidecar/fanout"
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
	"go.uber.org/zap"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestReadConfigMap(t *testing.T) {
	testCases := []struct {
		name        string
		createDir   bool
		config      string
		expected    multichannelfanout.Config
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
						- callDomain: event-changer.default.svc.cluster.local
						  toDomain: message-dumper-bar.default.svc.cluster.local
						- callDomain: message-dumper-foo.default.svc.cluster.local
						- toDomain: message-dumper-bar.default.svc.cluster.local
				  - namespace: default
					name: c2
					fanoutConfig:
					  subscriptions:
						- toDomain: message-dumper-foo.default.svc.cluster.local
				  - namespace: other
					name: c3
					fanoutConfig:
					  subscriptions:
						- toDomain: message-dumper-foo.default.svc.cluster.local
				`,
			expected: multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						Namespace: "default",
						Name:      "c1",
						FanoutConfig: fanout.Config{
							Subscriptions: []fanout.Subscription{
								{
									CallDomain: "event-changer.default.svc.cluster.local",
									ToDomain:   "message-dumper-bar.default.svc.cluster.local",
								},
								{
									CallDomain: "message-dumper-foo.default.svc.cluster.local",
								},
								{
									ToDomain: "message-dumper-bar.default.svc.cluster.local",
								},
							},
						},
					},
					{
						Namespace: "default",
						Name:      "c2",
						FanoutConfig: fanout.Config{
							Subscriptions: []fanout.Subscription{
								{
									ToDomain: "message-dumper-foo.default.svc.cluster.local",
								},
							},
						},
					},
					{
						Namespace: "other",
						Name:      "c3",
						FanoutConfig: fanout.Config{
							Subscriptions: []fanout.Subscription{
								{
									ToDomain: "message-dumper-foo.default.svc.cluster.local",
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
			writeConfig(t, dir, tc.config)
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

func TestNewHandler(t *testing.T) {
	testCases := []struct {
		name      string
		createDir bool
		config    string
		expectErr bool
	}{
		{
			name:      "dir does not exist",
			createDir: false,
			expectErr: true,
		},
		{
			name:      "duplicate channel key",
			createDir: true,
			config: `
				channelConfigs:
				  - namespace: default
					name: duplicate
				  - namespace: default
					name: duplicate
				`,
			expectErr: true,
		},
		{
			name:      "success",
			createDir: true,
			config: `
				channelConfigs: []
				`,
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
			writeConfig(t, dir, tc.config)
			cmh, err := NewHandler(zap.NewNop(), dir, &fake.ClientFactory{})
			if err == nil {
				// This is not yet about the logic of the test, just ensuring we don't accidentally
				// leave the channel open, which will leave the watcher running.
				defer close(cmh.(*configMapHandler).watcherStopCh)
			}
			if tc.expectErr {
				if err == nil {
					t.Errorf("Expected an error, actually nil")
				}
				return
			}
		})
	}
}

func TestServeHTTP(t *testing.T) {
	testCases := []struct {
		name                  string
		initialConfig         string
		updatedConfig         string
		expectedInitialDomain string
		expectedUpdatedDomain string
	}{
		{
			name: "send to config",
			initialConfig: `
				channelConfigs:
				  - namespace: default
					name: c1
					fanoutConfig:
						subscriptions:
						  - toDomain: initial-domain
				`,
			expectedInitialDomain: "initial-domain",
		},
		{
			name: "change config",
			initialConfig: `
				channelConfigs:
				  - namespace: default
					name: c1
					fanoutConfig:
						subscriptions:
						  - toDomain: initial-domain
				`,
			expectedInitialDomain: "initial-domain",
			updatedConfig: `
				channelConfigs:
				  - namespace: default
					name: c1
					fanoutConfig:
						subscriptions:
						  - toDomain: updated-domain
				`,
			expectedUpdatedDomain: "updated-domain",
		},
		{
			name: "bad config update -- keeps serving old config",
			initialConfig: `
				channelConfigs:
				  - namespace: default
					name: c1
					fanoutConfig:
						subscriptions:
						  - toDomain: initial-domain
				`,
			expectedInitialDomain: "initial-domain",
			updatedConfig: `
				channelConfigs:
				  - namespace: default
					name: c1
					fanoutConfig:
						subscriptions:
						  - toDomain: updated-domain
				  # Duplicate namespace/name
				  - namespace: default
					name: c1
					fanoutConfig:
						subscriptions:
						  - toDomain: updated-domain
				`,
			expectedUpdatedDomain: "initial-domain",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := createTempDir(t)
			defer os.RemoveAll(dir)
			writeConfig(t, dir, tc.initialConfig)

			cf := &fake.ClientFactory{
				Resp: []*http.Response{
					{
						StatusCode: http.StatusOK,
						Body:       body(""),
					},
				},
			}
			cmh, _ := NewHandler(zap.NewNop(), dir, cf)

			w := httptest.NewRecorder()
			cmh.ServeHTTP(w, makeRequest("default", "c1"))
			if w.Result().StatusCode != http.StatusOK {
				t.Errorf("Unexpected initial status code: %v", w.Result().StatusCode)
			}
			if initialHost := cf.GetRequests()[0].Host; initialHost != tc.expectedInitialDomain {
				t.Errorf("Sent initial request to wrong domain. Expected: '%v'. Actual '%v'", tc.expectedInitialDomain, initialHost)
			}

			if tc.updatedConfig != "" {
				writeConfig(t, dir, tc.updatedConfig)
				// The watcher is running in another routine, give it some time to notice the
				// change.
				time.Sleep(100 * time.Millisecond)
				w = httptest.NewRecorder()
				cmh.ServeHTTP(w, makeRequest("default", "c1"))
				if w.Result().StatusCode != http.StatusOK {
					t.Errorf("Unexpected updated status code: %v", w.Result().StatusCode)
				}
				if updatedDomain := cf.GetRequests()[1].Host; updatedDomain != tc.expectedUpdatedDomain {
					t.Errorf("Sent updated request to wrong domain. Expected: '%v'. Actual: '%v'", tc.expectedUpdatedDomain, updatedDomain)
				}
			}
		})
	}
}

func createTempDir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "configMapHandlerTest")
	if err != nil {
		t.Errorf("Unable to make temp directory: %v", err)
	}
	return dir
}

func writeConfig(t *testing.T, dir, config string) {
	if config != "" {
		// Golang editors tend to replace leading spaces with tabs. YAML is left whitespace
		// sensitive, so let's replace the tabs with spaces.
		leftSpaceConfig := strings.Replace(config, "\t", "    ", -1)
		err := ioutil.WriteFile(fmt.Sprintf("%s/%s", dir, multiChannelFanoutConfigKey), []byte(leftSpaceConfig), 0700)
		if err != nil {
			t.Errorf("Problem writing the config file: %v", err)
		}
	}
}

func body(body string) io.ReadCloser {
	return ioutil.NopCloser(strings.NewReader(body))
}

func makeRequest(namespace, name string) *http.Request {
	r := httptest.NewRequest("POST", "http://example/", body(""))
	r.Header.Add(multichannelfanout.ChannelKeyHeader, multichannelfanout.MakeChannelKey(namespace, name))
	return r
}
