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
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/knative/eventing/pkg/sidecar/configmap/parse"
	"github.com/knative/eventing/pkg/sidecar/fanout"
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
	"github.com/knative/eventing/pkg/sidecar/swappable"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"go.uber.org/atomic"
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

const (
	replaceDomain = "replaceDomain"
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

func TestServeHTTP(t *testing.T) {
	testCases := []struct {
		name                       string
		initialConfig              string
		updatedConfig              string
		initialRequests            int32
		initialRequestsAfterUpdate int32
		updateRequests             int32
	}{
		{
			name: "send to config",
			initialConfig: `
				channelConfigs:
				  - namespace: default
					name: c1
					fanoutConfig:
						subscriptions:
						  - sinkableDomain: replaceDomain
				`,
			initialRequests: 1,
		},
		{
			name: "change config",
			initialConfig: `
				channelConfigs:
				  - namespace: default
					name: c1
					fanoutConfig:
						subscriptions:
						  - sinkableDomain: replaceDomain
				`,
			initialRequests: 1,
			updatedConfig: `
				channelConfigs:
				  - namespace: default
					name: c1
					fanoutConfig:
						subscriptions:
						  - sinkableDomain: replaceDomain
				`,
			updateRequests: 1,
		},
		{
			name: "bad config update -- keeps serving old config",
			initialConfig: `
				channelConfigs:
				  - namespace: default
					name: c1
					fanoutConfig:
						subscriptions:
						  - sinkableDomain: replaceDomain
				`,
			initialRequests: 1,
			updatedConfig: `
				channelConfigs:
				  - namespace: default
					name: c1
					fanoutConfig:
						subscriptions:
						  - sinkableDomain: replaceDomain
				  # Duplicate namespace/name
				  - namespace: default
					name: c1
					fanoutConfig:
						subscriptions:
						  - sinkableDomain: replaceDomain
				`,
			initialRequestsAfterUpdate: 2,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			initialHandler := &fakeHandler{}
			initialServer := httptest.NewServer(initialHandler)
			defer initialServer.Close()
			updateHandler := &fakeHandler{}
			updateServer := httptest.NewServer(updateHandler)
			defer updateServer.Close()

			dir := createTempDir(t)
			defer os.RemoveAll(dir)
			writeConfig(t, dir, replaceDomains(tc.initialConfig, initialServer.URL[7:]))

			sh, err := swappable.NewEmptyHandler(zap.NewNop())
			if err != nil {
				t.Errorf("Unexpected error making swappable.Handler: %+v", err)
			}
			_, err = NewConfigMapWatcher(zap.NewNop(), dir, sh.UpdateConfig)
			if err != nil {
				t.Errorf("Unexpected error making filesystem.configMapWatcher")
			}

			w := httptest.NewRecorder()
			sh.ServeHTTP(w, makeRequest("default", "c1"))
			if w.Result().StatusCode != http.StatusOK {
				t.Errorf("Unexpected initial status code: %v", w.Result().StatusCode)
			}
			if tc.initialRequests != initialHandler.requests.Load() {
				t.Errorf("Incorrect initial request count. Expected %v. Actual %v.",
					tc.initialRequests, initialHandler.requests.Load())
			}

			if tc.updatedConfig != "" {
				writeConfig(t, dir, replaceDomains(tc.updatedConfig, updateServer.URL[7:]))
				// The watcher is running in another routine, give it some time to notice the
				// change.
				time.Sleep(100 * time.Millisecond)
				w = httptest.NewRecorder()
				sh.ServeHTTP(w, makeRequest("default", "c1"))
				if w.Result().StatusCode != http.StatusOK {
					t.Errorf("Unexpected updated status code: %v", w.Result().StatusCode)
				}
				if tc.updateRequests != updateHandler.requests.Load() {
					t.Errorf("Incorrect update request count. Expected %v. Actual %v.", tc.updateRequests, updateHandler.requests.Load())
				}
				if tc.initialRequestsAfterUpdate != 0 && tc.initialRequestsAfterUpdate != initialHandler.requests.Load() {
					t.Errorf("Incorrect requests to initialHandler after config update. Expected %v, actual %v",
						tc.initialRequestsAfterUpdate, initialHandler.requests.Load())
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
		err := ioutil.WriteFile(fmt.Sprintf("%s/%s", dir, parse.MultiChannelFanoutConfigKey), []byte(leftSpaceConfig), 0700)
		if err != nil {
			t.Errorf("Problem writing the config file: %v", err)
		}
	}
}

func body(body string) io.ReadCloser {
	return ioutil.NopCloser(strings.NewReader(body))
}

func makeRequest(namespace, name string) *http.Request {
	r := httptest.NewRequest("POST", fmt.Sprintf("http://%s.%s/", name, namespace), body(""))
	return r
}

type fakeHandler struct {
	requests atomic.Int32
}

func (h *fakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	w.WriteHeader(http.StatusOK)
	h.requests.Inc()
}

func replaceDomains(config, replacement string) string {
	return strings.Replace(config, replaceDomain, replacement, -1)
}
