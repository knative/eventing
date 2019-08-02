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

package cronjobsource

import (
	"fmt"
	"os"
	"sync"
)

// EnvLookup returns the value of the associated environment variable. It only does the lookup once.
// If the environment variable is undefined, then every call to GetValue() will panic.
type EnvLookup interface {
	// GetValue returns the value of the associated environment variable or panic if the environment
	// variable is not defined.
	GetValue() string
}

// envLookup looks up an environment variable on first usage. If the environment variable is not
// defined, then every call will panic.
type envLookup struct {
	once  sync.Once
	panic error
	key   string
	value string
}

var _ EnvLookup = (*envLookup)(nil)

func newEnvLookup(key string) EnvLookup {
	return &envLookup{
		key: key,
	}
}

// GetValue implements EnvLookup.GetValue.
func (e *envLookup) GetValue() string {
	e.once.Do(func() {
		value, defined := os.LookupEnv(e.key)
		if !defined {
			e.panic = fmt.Errorf("required environment variable %q not defined", e.key)
		} else {
			e.value = value
		}
	})
	if e.panic != nil {
		panic(e.panic)
	}
	return e.value
}
