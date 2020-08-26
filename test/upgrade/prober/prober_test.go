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

type envValue uint

const (
	EnvFalse envValue = iota
	EnvTrue
	EnvUnset
)

func TestNewConfig(t *testing.T) {
	envname := "E2E_UPGRADE_TESTS_SERVING_USE"
	suite := []struct {
		env envValue
		out bool
	}{
		{EnvFalse, false},
		{EnvTrue, true},
		{EnvUnset, false},
	}
	for _, s := range suite {
		t.Run(fmt.Sprintf("env=%v,out=%t", s.env, s.out), func(t *testing.T) {
			if s.env != EnvUnset {
				val := "false"
				if s.env == EnvTrue {
					val = "true"
				}
				assert.NoError(t, os.Setenv(envname, val))
				defer func() { assert.NoError(t, os.Unsetenv(envname)) }()
			}
			config := NewConfig("test-ns")

			assert.Equal(t, s.out, config.Serving.Use)
			assert.True(t, config.Serving.ScaleToZero)
		})
	}
}
