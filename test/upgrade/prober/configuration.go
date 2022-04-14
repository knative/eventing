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
	"context"
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/eventing/test/upgrade/prober/sut"
	watholaconfig "knative.dev/eventing/test/upgrade/prober/wathola/config"
	"knative.dev/eventing/test/upgrade/prober/wathola/forwarder"
	"knative.dev/eventing/test/upgrade/prober/wathola/receiver"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgTest "knative.dev/pkg/test"
	pkgupgrade "knative.dev/pkg/test/upgrade"
	"sigs.k8s.io/yaml"
)

const (
	defaultConfigName       = "wathola-config"
	defaultConfigMountPoint = "/etc/wathola"
	defaultConfigFilename   = "config.yaml"
	defaultHealthEndpoint   = "/healthz"

	// Silence will suppress notification about event duplicates.
	Silence DuplicateAction = "silence"
	// Warn will issue a warning message in case of a event duplicate.
	Warn DuplicateAction = "warn"
	// Error will fail test with an error in case of event duplicate.
	Error DuplicateAction = "error"

	prefix = "eventing_upgrade_tests"
)

// ErrInvalidConfig is returned when the configuration is invalid.
var ErrInvalidConfig = errors.New("invalid config")

// DuplicateAction is the action to take in case of duplicated events
type DuplicateAction string

// Config represents a configuration for prober.
type Config struct {
	Wathola
	Interval     time.Duration
	Serving      ServingConfig
	FailOnErrors bool
	OnDuplicate  DuplicateAction
	Ctx          context.Context
}

// Wathola represents options related strictly to wathola testing tool.
type Wathola struct {
	*watholaconfig.Config
	ImageResolver
	SystemUnderTest sut.SystemUnderTest
	HealthEndpoint  string
}

// ImageResolver will resolve the container image for given component.
type ImageResolver func(component string) string

// ServingConfig represents a options for serving test component (wathola-forwarder).
type ServingConfig struct {
	Use         bool
	ScaleToZero bool
}

// NewConfigOrFail will create a prober.Config or fail trying.
func NewConfigOrFail(c pkgupgrade.Context) *Config {
	cfg, err := NewConfig()
	if err != nil {
		c.T.Fatal(err)
	}
	return cfg
}

// NewConfig will create a prober.Config or return error.
func NewConfig() (*Config, error) {
	config := &Config{
		Interval:     Interval,
		FailOnErrors: true,
		OnDuplicate:  Warn,
		Ctx:          context.Background(),
		Serving: ServingConfig{
			Use:         false,
			ScaleToZero: true,
		},
		Wathola: Wathola{
			Config:          watholaconfig.Defaults(),
			ImageResolver:   pkgTest.ImagePath,
			HealthEndpoint:  defaultHealthEndpoint,
			SystemUnderTest: sut.NewDefault(),
		},
	}

	if err := envconfig.Process(prefix, config); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	return config, nil
}

func (p *prober) deployConfiguration() {
	sc := sut.Context{
		Ctx:    p.config.Ctx,
		Log:    p.log,
		Client: p.client,
	}
	ref := resources.KnativeRefForService(receiver.Name, p.client.Namespace)
	if p.config.Serving.Use {
		ref = resources.KnativeRefForKservice(forwarder.Name, p.client.Namespace)
	}
	dest := duckv1.Destination{Ref: ref}
	s := p.config.SystemUnderTest
	endpoint := s.Deploy(sc, dest)
	p.client.Cleanup(func() {
		if tr, ok := s.(sut.HasTeardown); ok {
			tr.Teardown(sc)
		}
	})

	p.deployConfig(endpoint)
}

func (p *prober) deployConfig(endpoint interface{}) {
	name := defaultConfigName
	p.log.Infof("Deploying config map: \"%s/%s\"", p.client.Namespace, name)
	configData := p.compileConfigData(*p.config.Wathola.Config, endpoint, p.client.TracingCfg)
	p.client.CreateConfigMapOrFail(name, p.client.Namespace, map[string]string{
		defaultConfigFilename: configData,
	})
}

func (p *prober) compileConfigData(cfg watholaconfig.Config, endpoint interface{}, tracingConfig string) string {
	cfg.Sender.Address = endpoint
	cfg.Forwarder.Target = p.forwarderTarget()
	cfg.TracingConfig = tracingConfig
	bytes, err := yaml.Marshal(cfg)
	p.ensureNoError(err)
	return string(bytes)
}

func (p *prober) forwarderTarget() string {
	uri := apis.HTTP(receiver.Name + "." + p.client.Namespace + ".svc")
	return uri.String()
}
