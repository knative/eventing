/*
Copyright 2019 The Knative Authors

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

package utils

import (
	"testing"

	"go.uber.org/zap/zapcore"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"

	"github.com/google/go-cmp/cmp"
)

func TestMetricsOptions(t *testing.T) {
	testCases := map[string]struct {
		opts    *metrics.ExporterOptions
		want    string
		wantErr string
	}{
		"nil": {
			opts:    nil,
			want:    "",
			wantErr: "base64 metrics string is empty",
		},
		"happy": {
			opts: &metrics.ExporterOptions{
				Domain:         "domain",
				Component:      "component",
				PrometheusPort: 9090,
				ConfigMap: map[string]string{
					"foo":   "bar",
					"boosh": "kakow",
				},
			},
			want: "eyJEb21haW4iOiJkb21haW4iLCJDb21wb25lbnQiOiJjb21wb25lbnQiLCJQcm9tZXRoZXVzUG9ydCI6OTA5MCwiQ29uZmlnTWFwIjp7ImJvb3NoIjoia2Frb3ciLCJmb28iOiJiYXIifX0=",
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			base64, err := MetricsOptionsToBase64(tc.opts)
			if err != nil {
				t.Errorf("error while converting metrics config to base64: %v", err)
			}
			// Test to base64.
			{
				want := tc.want
				got := base64
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("unexpected (-want, +got) = %v", diff)
					t.Log(got)
				}
			}
			// Test to options.
			{
				want := tc.opts
				got, gotErr := Base64ToMetricsOptions(base64)

				if gotErr != nil {
					if diff := cmp.Diff(tc.wantErr, gotErr.Error()); diff != "" {
						t.Errorf("unexpected err (-want, +got) = %v", diff)
					}
				} else if tc.wantErr != "" {
					t.Errorf("expected err %v", tc.wantErr)
				}

				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("unexpected (-want, +got) = %v", diff)
					t.Log(got)
				}
			}
		})
	}
}

func TestLoggingConfig(t *testing.T) {
	testCases := map[string]struct {
		cfg     *logging.Config
		want    string
		wantErr string
	}{
		"nil": {
			cfg:     nil,
			want:    "",
			wantErr: "base64 logging string is empty",
		},
		"happy": {
			cfg: &logging.Config{
				LoggingConfig: "{}",
				LoggingLevel:  map[string]zapcore.Level{},
			},
			want: "eyJ6YXAtbG9nZ2VyLWNvbmZpZyI6Int9In0=",
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			base64, err := LoggingConfigToBase64(tc.cfg)
			if err != nil {
				t.Errorf("error while converting logging config to base64: %v", err)
			}
			// Test to base64.
			{
				want := tc.want
				got := base64
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("unexpected (-want, +got) = %v", diff)
					t.Log(got)
				}
			}
			// Test to config.
			if tc.cfg != nil {
				want := tc.cfg
				got, gotErr := Base64ToLoggingConfig(base64)

				if gotErr != nil {
					if diff := cmp.Diff(tc.wantErr, gotErr.Error()); diff != "" {
						t.Errorf("unexpected err (-want, +got) = %v", diff)
					}
				} else if tc.wantErr != "" {
					t.Errorf("expected err %v", tc.wantErr)
				}

				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("unexpected (-want, +got) = %v", diff)
					t.Log(got)
				}
			}
		})
	}
}
