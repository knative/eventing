/*
 * Copyright 2020 The Knative Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * 	http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package prober

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"runtime"
	"text/template"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/network"
	pkgTest "knative.dev/pkg/test"
	pkgupgrade "knative.dev/pkg/test/upgrade"

	"knative.dev/eventing/test/lib/resources"
	"knative.dev/eventing/test/upgrade/prober/sut"
	"knative.dev/eventing/test/upgrade/prober/wathola/forwarder"
	"knative.dev/eventing/test/upgrade/prober/wathola/receiver"
)

const (
	defaultConfigName        = "wathola-config"
	defaultConfigHomedirPath = ".config/wathola"
	defaultHomedir           = "/home/nonroot"
	defaultConfigFilename    = "config.toml"
	defaultHealthEndpoint    = "/healthz"

	// Silence will suppress notification about event duplicates.
	Silence DuplicateAction = "silence"
	// Warn will issue a warning message in case of a event duplicate.
	Warn DuplicateAction = "warn"
	// Error will fail test with an error in case of event duplicate.
	Error DuplicateAction = "error"

	prefix = "eventing_upgrade_tests"

	forwarderTargetFmt = "http://" + receiver.Name + ".%s.svc.%s"

	defaultTraceExportLimit = 100
)

var (
	// ErrInvalidConfig is returned when the configuration is invalid.
	ErrInvalidConfig = errors.New("invalid config")
)

// DuplicateAction is the action to take in case of duplicated events
type DuplicateAction string

// Config represents a configuration for prober.
type Config struct {
	Wathola
	Interval         time.Duration
	Serving          ServingConfig
	FailOnErrors     bool
	OnDuplicate      DuplicateAction
	Ctx              context.Context
	TraceExportLimit int
}

// Wathola represents options related strictly to wathola testing tool.
type Wathola struct {
	ConfigToml
	ImageResolver
	SystemUnderTest sut.SystemUnderTest
	HealthEndpoint  string
}

// ImageResolver will resolve the container image for given component.
type ImageResolver func(component string) string

// ConfigToml represents options of wathola config toml file.
type ConfigToml struct {
	// ConfigTemplate is a template file that will be compiled to the configmap
	ConfigTemplate   string
	ConfigMapName    string
	ConfigMountPoint string
	ConfigFilename   string
}

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
		TraceExportLimit: defaultTraceExportLimit,
		Wathola: Wathola{
			ImageResolver: pkgTest.ImagePath,
			ConfigToml: ConfigToml{
				ConfigTemplate:   defaultConfigFilename,
				ConfigMapName:    defaultConfigName,
				ConfigMountPoint: fmt.Sprintf("%s/%s", defaultHomedir, defaultConfigHomedirPath),
				ConfigFilename:   defaultConfigFilename,
			},
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

	p.deployConfigToml(endpoint)
}

func (p *prober) deployConfigToml(endpoint interface{}) {
	name := p.config.ConfigMapName
	p.log.Infof("Deploying config map: \"%s/%s\"", p.client.Namespace, name)
	configData := p.compileTemplate(p.config.ConfigTemplate, endpoint, p.client.ObservabilityCfg)
	p.client.CreateConfigMapOrFail(name, p.client.Namespace, map[string]string{
		p.config.ConfigFilename: configData,
	})
}

func (p *prober) compileTemplate(templateName string, endpoint interface{}, observabilityConfig string) string {
	_, filename, _, _ := runtime.Caller(0)
	templateFilepath := path.Join(path.Dir(filename), templateName)
	templateBytes, err := os.ReadFile(templateFilepath)
	p.ensureNoError(err)
	tmpl, err := template.New(templateName).Parse(string(templateBytes))
	p.ensureNoError(err)
	var buff bytes.Buffer
	data := struct {
		*Config
		Endpoint            interface{}
		ObservabilityConfig string
		ForwarderTarget     string
	}{
		p.config,
		endpoint,
		observabilityConfig,
		fmt.Sprintf(forwarderTargetFmt, p.client.Namespace, network.GetClusterDomainName()),
	}
	p.ensureNoError(tmpl.Execute(&buff, data))
	return buff.String()
}
