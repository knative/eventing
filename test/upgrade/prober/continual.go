/*
Copyright 2021 The Knative Authors

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

package prober

import (
	testlib "knative.dev/eventing/test/lib"
	pkgupgrade "knative.dev/pkg/test/upgrade"
)

// Configurator will customize default config.
type Configurator func(config *Config) error

// ContinualVerificationOptions holds options for NewContinualVerification func.
type ContinualVerificationOptions struct {
	Configurators []Configurator
	ClientOptions []testlib.SetupClientOption
}

// NewContinualVerification will create a new continual verification operation
// that will verify that SUT is propagating events well through the test
// process. It's a general pattern that can be used to validate upgrade or
// performance testing.
func NewContinualVerification(
	name string, opts ContinualVerificationOptions,
) pkgupgrade.BackgroundOperation {
	var config *Config
	var runner Runner
	setup := func(c pkgupgrade.Context) {
		config = NewConfigOrFail(c)
		if len(opts.Configurators) > 0 {
			for _, fn := range opts.Configurators {
				err := fn(config)
				if err != nil {
					c.T.Fatal(err)
				}
			}
		}
		runner = NewRunner(config, opts.ClientOptions...)
		runner.Setup(c)
	}
	verify := func(c pkgupgrade.Context) {
		runner.Verify(c)
	}
	return pkgupgrade.NewBackgroundVerification(name, setup, verify)
}
