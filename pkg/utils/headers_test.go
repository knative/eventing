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

package utils

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPassThroughHeaders(t *testing.T) {

	testCases := map[string]struct {
		additionalHeaders            http.Header
		expectedPassedThroughHeaders http.Header
	}{
		"valid headers pass through": {
			additionalHeaders: map[string][]string{
				"not":                       {"passed", "through"},
				"x-requEst-id":              {"1234"},
				"nor":                       {"this-one"},
				"knatIve-will-pass-through": {"true", "always"},
				"nope":                      {"nada"},
				"X-B3-Spanid":               {"5678"},
			},
			expectedPassedThroughHeaders: map[string][]string{
				"x-requEst-id":              {"1234"},
				"knatIve-will-pass-through": {"true", "always"},
				"X-B3-Spanid":               {"5678"},
			},
		},
		"nothing passes through": {
			additionalHeaders: map[string][]string{
				"not":  {"passed", "through"},
				"nor":  {"this-one"},
				"nope": {"nada"},
			},
			expectedPassedThroughHeaders: map[string][]string{},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			headers := PassThroughHeaders(tc.additionalHeaders)
			assert.Equal(t, tc.expectedPassedThroughHeaders, headers)
		})
	}
}
