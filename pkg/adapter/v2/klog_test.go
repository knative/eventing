/*
Copyright 2026 The Knative Authors

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

package adapter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/klog/v2"
)

func TestSetupKlogLevel(t *testing.T) {
	testCases := []struct {
		name  string
		level int
	}{
		{
			name:  "Level 0 - minimal",
			level: 0,
		},
		{
			name:  "Level 2 - default",
			level: 2,
		},
		{
			name:  "Level 4 - debug",
			level: 4,
		},
		{
			name:  "Level 7 - verbose",
			level: 7,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			SetupKlogLevel(tc.level)

			// Verify that the global klog verbosity is actually set
			// klog.V(n).Enabled() returns true if the global level is >= n
			if tc.level > 0 {
				assert.True(t, klog.V(klog.Level(tc.level)).Enabled(), "klog verbosity %d should be enabled", tc.level)
			}

			// Verify that a higher level is NOT enabled (unless we are at max level)
			// This ensures we aren't just setting it to max everywhere
			if tc.level < 10 {
				assert.False(t, klog.V(klog.Level(tc.level+1)).Enabled(), "klog verbosity %d should NOT be enabled", tc.level+1)
			}
		})
	}
}
