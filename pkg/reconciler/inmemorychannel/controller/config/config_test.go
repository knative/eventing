/*
Copyright 2020 The Knative Authors

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

package config

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	configmaptesting "knative.dev/pkg/configmap/testing"
	logtesting "knative.dev/pkg/logging/testing"

	"knative.dev/eventing/pkg/kncloudevents"
)

func TestGetConfig(t *testing.T) {
	for _, tt := range []struct {
		name string
		file string
		want EventDispatcherConfig
		keys []string
	}{
		{
			name: "All keys are configured",
			file: "config-event-dispatcher-1",
			want: EventDispatcherConfig{
				ConnectionArgs: kncloudevents.ConnectionArgs{
					MaxIdleConns:        20,
					MaxIdleConnsPerHost: 10,
				},
			},
			keys: []string{"MaxIdleConnections", "MaxIdleConnectionsPerHost"},
		},
		{
			name: "Only MaxIdleConnections is configured",
			file: "config-event-dispatcher-2",
			want: EventDispatcherConfig{
				ConnectionArgs: kncloudevents.ConnectionArgs{
					MaxIdleConns:        20,
					MaxIdleConnsPerHost: defaultMaxIdleConnectionsPerHost,
				},
			},
			keys: []string{"MaxIdleConnections"},
		},
		{
			name: "Only MaxIdleConnectionsPerHost is configured",
			file: "config-event-dispatcher-3",
			want: EventDispatcherConfig{
				ConnectionArgs: kncloudevents.ConnectionArgs{
					MaxIdleConns:        defaultMaxIdleConnections,
					MaxIdleConnsPerHost: 10,
				},
			},
			keys: []string{"MaxIdleConnectionsPerHost"},
		},
		{
			name: "Empty configmap",
			file: "config-event-dispatcher-4",
			want: EventDispatcherConfig{
				ConnectionArgs: kncloudevents.ConnectionArgs{
					MaxIdleConns:        defaultMaxIdleConnections,
					MaxIdleConnsPerHost: defaultMaxIdleConnectionsPerHost,
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := NewEventDispatcherConfigStore(logtesting.TestLogger(t))
			configmap := configmaptesting.ConfigMapFromTestFile(t, tt.file, tt.keys...)
			store.OnConfigChanged(configmap)
			got := store.GetConfig()
			if dif := cmp.Diff(got, tt.want); dif != "" {
				t.Errorf("EventDispatcherConfig mismatch. \n got: %+v \n, want: %+v \n, dif: %+v \n", got, tt.want, dif)
			}
		})
	}
}
