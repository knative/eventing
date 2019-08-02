/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Veroute.on 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cronjobsource

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
)

// FinishedEnvLookup creates an envLookup that will return the specified value.
func FinishedEnvLookup(value string) EnvLookup {
	e := &envLookup{
		value: value,
	}
	e.once.Do(func() {})
	return e
}

// PanickingEnvLookup creates an envLookup that will panic with the specified error.
func PanickingEnvLookup(err error) EnvLookup {
	e := &envLookup{
		panic: err,
	}
	e.once.Do(func() {})
	return e
}

func TestEnvLookup_GetValue(t *testing.T) {
	testCases := map[string]struct {
		setEnv bool
		value  string
	}{
		"not set": {},
		"empty value": {
			setEnv: true,
		},
		"real value": {
			setEnv: true,
			value:  "real value",
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			defer func() {
				r := recover()
				if tc.setEnv {
					if r != nil {
						t.Errorf("Panic detected when the env should have been set: %v", r)
					}
				} else {
					if r == nil {
						t.Errorf("Panic not detected when the env should not have been set")
					}
				}
			}()
			k := fmt.Sprintf("random-key-%s", uuid.New().String())
			if tc.setEnv {
				if err := os.Setenv(k, tc.value); err != nil {
					t.Fatalf("Error setting env %q: %v", k, err)
				}
				defer func() {
					if err := os.Unsetenv(k); err != nil {
						t.Fatalf("Error unsetting env %q: %v", k, err)
					}
				}()
			}
			e := newEnvLookup(k)
			if v := e.GetValue(); v != tc.value {
				t.Errorf("Unexpected value. Expected %q. Actually %q", tc.value, v)
			}
		})
	}
}

func TestEnvLookup_GetValue_RepeatedPanics(t *testing.T) {
	k := fmt.Sprintf("random-key-%s", uuid.New().String())
	e := newEnvLookup(k)
	for i := 0; i < 10; i++ {
		// Run inside a function to capture the panic.
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("Expected a non-nil panic. Was nil at iteration %d", i)
				}
			}()
			e.GetValue()
		}()
	}
}
