/*
Copyright 2025 The Knative Authors

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

package observability

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFromMap(t *testing.T) {
	configWithOverride := DefaultConfig()
	configWithOverride.EnableSinkEventErrorReporting = true

	testCases := map[string]struct {
		m                map[string]string
		expectParseError bool
		want             *Config
	}{
		"no map": {
			m: nil,
		},
		"valid keys and values": {
			m: map[string]string{
				EnableSinkEventErrorReportingKey: "true",
			},
			want: configWithOverride,
		},
		"valid keys, invalid sink event error reporting value": {
			m: map[string]string{
				EnableSinkEventErrorReportingKey: "notabool",
			},
		},
	}

	for tName, tCase := range testCases {
		t.Run(tName, func(t *testing.T) {
			t.Parallel()

			got, err := NewFromMap(tCase.m)

			if tCase.expectParseError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tCase.want, got, "with no data, NewFromMap should return default config")
		})
	}
}
