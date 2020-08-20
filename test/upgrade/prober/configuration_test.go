/*
 * Copyright 2020 The Knative Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package prober

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvflag(t *testing.T) {
	envname := "TestEnvflag"
	suite := []struct {
		env          string
		envSet       bool
		defaultValue bool
		out          bool
	}{
		{"", false, true, true},
		{"", true, true, false},
		{"yes", true, false, true},
		{"y", true, false, false},
		{"y", true, true, false},
		{"true", true, false, true},
		{"true", true, true, true},
		{"enable", true, false, true},
		{"enabled", true, false, false},
	}
	for _, s := range suite {
		t.Run(fmt.Sprintf("env(%t)=%q,def=%t", s.envSet, s.env, s.defaultValue), func(t *testing.T) {
			if s.envSet {
				assert.NoError(t, os.Setenv(envname, s.env))
				defer func() { assert.NoError(t, os.Unsetenv(envname)) }()
			}
			result := envflag(envname, s.defaultValue)

			assert.Equal(t, s.out, result)
		})
	}
}
