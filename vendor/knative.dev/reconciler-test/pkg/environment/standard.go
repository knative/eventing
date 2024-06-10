/*
Copyright 2022 The Knative Authors
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

package environment

import (
	"context"
	"flag"

	"k8s.io/client-go/rest"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"

	"knative.dev/reconciler-test/pkg/images/file"
	testlog "knative.dev/reconciler-test/pkg/logging"
)

// Flags is used to pass flags implementation.
type Flags interface {
	// Get returns the flag set object on which specific flags are registered.
	// After registration is complete, the Parse method should be called.
	Get(ctx context.Context) *flag.FlagSet

	// Parse invokes the processing of underlying inputs that will update the
	// previously returned flag.FlagSet object. Thi method should be called after
	// the specific flags has been defined on object returned from the Get method.
	Parse(ctx context.Context) error
}

// Configuration holds a configuration options for standard GlobalEnvironment.
type Configuration struct {
	Flags
	context.Context
	*rest.Config
}

// ConfigurationOption allows to reconfigure default options, by returning
// modified configuration.
type ConfigurationOption func(Configuration) Configuration

// NewStandardGlobalEnvironment will create a new global environment in a
// standard way. The Kube client will be initialized within the
// context.Context for later use.
func NewStandardGlobalEnvironment(opts ...ConfigurationOption) GlobalEnvironment {
	return NewStandardGlobalEnvironmentWithRestConfig(nil, opts...)
}

// NewStandardGlobalEnvironment will create a new global environment in a
// standard way. The Kube client will be initialized within the
// context.Context for later use. It uses the provided rest config
// when creating the informers.
func NewStandardGlobalEnvironmentWithRestConfig(cfg *rest.Config, opts ...ConfigurationOption) GlobalEnvironment {
	opts = append(opts, initIstioFlags())
	config := resolveConfiguration(opts, cfg)
	ctx := testlog.NewContext(config.Context)

	// environment.InitFlags registers state, level and feature filter flags.
	InitFlags(config.Flags.Get(ctx))

	// We get a chance to parse flags to include the framework flags for the
	// framework as well as any additional flags included in the integration.
	if err := config.Flags.Parse(ctx); err != nil {
		logging.FromContext(ctx).Fatal(err)
	}

	if ipFilePath != nil && *ipFilePath != "" {
		ctx = withImageProducer(ctx, file.ImageProducer(*ipFilePath))
	}

	if testNamespace != nil && *testNamespace != "" {
		ctx = withNamespace(ctx, *testNamespace)
	}

	// EnableInjectionOrDie will enable client injection, this is used by the
	// testing framework for namespace management, and could be leveraged by
	// features to pull Kubernetes clients or the test environment out of the
	// context passed in the features.
	var startInformers func()

	if config.Config == nil {
		config.Config = injection.ParseAndGetRESTConfigOrDie()
	}
	// Respect user provided settings, but if omitted customize the default behavior.
	//
	// Use 20 times the default QPS and Burst to speed up testing since this client is used by
	// every running test.
	multiplier := 20
	if config.Config.QPS == 0 {
		config.Config.QPS = rest.DefaultQPS * float32(multiplier)
	}
	if config.Config.Burst == 0 {
		config.Config.Burst = rest.DefaultBurst * multiplier
	}

	ctx, startInformers = injection.EnableInjectionOrDie(ctx, config.Config)

	// global is used to make instances of Environments, NewGlobalEnvironment
	// is passing and saving the client injection enabled context for use later.
	return NewGlobalEnvironment(ctx, startInformers)
}

func resolveConfiguration(opts []ConfigurationOption, restCfg *rest.Config) Configuration {
	cfg := Configuration{
		Flags:   commandlineFlags{},
		Context: signals.NewContext(),
		Config:  restCfg,
	}
	for _, opt := range opts {
		cfg = opt(cfg)
	}
	return cfg
}

type commandlineFlags struct{}

func (c commandlineFlags) Get(ctx context.Context) *flag.FlagSet {
	return flag.CommandLine
}

func (c commandlineFlags) Parse(ctx context.Context) error {
	flag.Parse()
	return nil
}
