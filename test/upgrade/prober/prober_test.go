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

package prober_test

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"knative.dev/eventing/test/upgrade/prober"
)

const (
	servingEnvName = "EVENTING_UPGRADE_TESTS_SERVING_USE"
)

func TestNewConfig(t *testing.T) {
	unsetList := []string{
		servingEnvName,
	}

	for _, s := range createTestSuite() {
		t.Run(fmt.Sprintf("env=%#v", s.env), func(t *testing.T) {
			for envname, value := range s.env {
				assert.NoError(t, os.Setenv(envname, value))
			}
			defer func() {
				for _, envname := range unsetList {
					assert.NoError(t, os.Unsetenv(envname))
				}
			}()

			config, err := prober.NewConfig()
			if !errors.Is(err, s.err) {
				t.Fatalf("want err: %v, got err: %v", s.err, err)
			}
			if err != nil {
				return
			}
			assert.Equal(t, s.servingUse, config.Serving.Use)
			assert.True(t, config.Serving.ScaleToZero)
		})
	}
}

type testCase struct {
	servingUse bool
	env        map[string]string
	err        error
}

func createTestSuite() []testCase {
	return []testCase{
		createTestCase(func(c *testCase) {
			c.env = map[string]string{servingEnvName: "false"}
		}),
		createTestCase(func(c *testCase) {
			c.env = map[string]string{servingEnvName: "true"}
			c.servingUse = true
		}),
		createTestCase(func(c *testCase) {}),
		createTestCase(func(c *testCase) {
			c.env = map[string]string{
				servingEnvName: "gibberish",
			}
			c.err = prober.ErrInvalidConfig
		}),
	}
}

func createTestCase(overrides func(*testCase)) testCase {
	c := testCase{
		servingUse: false,
		env:        map[string]string{},
	}
	overrides(&c)
	return c
}
