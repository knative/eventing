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
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/wavesoftware/go-ensure"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/eventing/test/upgrade/prober/sut"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgTest "knative.dev/pkg/test"
	pkgupgrade "knative.dev/pkg/test/upgrade"
)

const (
	defaultConfigName        = "wathola-config"
	defaultConfigHomedirPath = ".config/wathola"
	defaultHomedir           = "/home/nonroot"
	defaultConfigFilename    = "config.toml"
	defaultHealthEndpoint    = "/healthz"
	defaultFinishedSleep     = 5 * time.Second

	Silence DuplicateAction = "silence"
	Warn    DuplicateAction = "warn"
	Error   DuplicateAction = "error"

	prefix = "eventing_upgrade_tests"

	// TODO(ksuszyns): Remove this val in next release
	// Deprecated: use 'eventing_upgrade_tests' prefix instead
	deprecatedPrefix = "e2e_upgrade_tests"
)

// DuplicateAction is the action to take in case of duplicated events
type DuplicateAction string

// Config represents a configuration for prober.
type Config struct {
	Wathola
	Interval      time.Duration
	FinishedSleep time.Duration
	Serving       ServingConfig
	FailOnErrors  bool
	OnDuplicate   DuplicateAction
	Ctx           context.Context
	// BrokerOpts holds opts for broker.
	// TODO(ksuszyns): Remove this opt in next release
	// Deprecated: use Wathola.SystemUnderTest instead.
	BrokerOpts []resources.BrokerOption
	// Namespace holds namespace in which test is about to be executed.
	// TODO(ksuszyns): Remove this opt in next release
	// Deprecated: namespace is about to be taken from testlib.Client created by
	// NewRunner
	Namespace string
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
	errSink := func(err error) {
		c.T.Fatal(err)
	}
	warnf := c.Log.Warnf
	return newConfig(errSink, warnf)
}

// NewConfig creates a new configuration object with default values filled in.
// Values can be influenced by kelseyhightower/envconfig with
// `eventing_upgrade_tests` prefix.
// TODO(ksuszyns): Remove this func in next release
// Deprecated: use NewContinualVerification or NewConfigOrFail
func NewConfig(namespace ...string) *Config {
	errSink := func(err error) {
		ensure.NoError(err)
	}
	warnf := func(template string, args ...interface{}) {
		_, err := fmt.Fprintf(os.Stderr, template, args)
		ensure.NoError(err)
	}
	config := newConfig(errSink, warnf)
	if len(namespace) > 0 {
		config.Namespace = strings.Join(namespace, ",")
	}
	return config
}

func newConfig(
	errSink func(err error), warnf func(template string, args ...interface{}),
) *Config {
	config := &Config{
		Interval:      Interval,
		FinishedSleep: defaultFinishedSleep,
		FailOnErrors:  true,
		OnDuplicate:   Warn,
		Ctx:           context.Background(),
		Serving: ServingConfig{
			Use:         false,
			ScaleToZero: true,
		},
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
		errSink(err)
	}

	// TODO(ksuszyns): Remove this block in next release
	for _, enventry := range os.Environ() {
		if strings.HasPrefix(strings.ToLower(enventry), deprecatedPrefix) {
			warnf(
				"DEPRECATED: using deprecated '%s' prefix. Use '%s' instead.",
				deprecatedPrefix, prefix)
			if err := envconfig.Process(deprecatedPrefix, config); err != nil {
				errSink(err)
			}
			break
		}
	}
	return config
}

func (p *prober) deployConfiguration() {
	sc := sut.Context{
		Ctx:    p.config.Ctx,
		Log:    p.log,
		Client: p.client,
	}
	ref := resources.KnativeRefForService(receiverName, p.client.Namespace)
	if p.config.Serving.Use {
		ref = resources.KnativeRefForKservice(forwarderName, p.client.Namespace)
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
	configData := p.compileTemplate(p.config.ConfigTemplate, endpoint)
	p.client.CreateConfigMapOrFail(name, p.client.Namespace, map[string]string{
		p.config.ConfigFilename: configData,
	})
}

func (p *prober) compileTemplate(templateName string, endpoint interface{}) string {
	_, filename, _, _ := runtime.Caller(0)
	templateFilepath := path.Join(path.Dir(filename), templateName)
	templateBytes, err := ioutil.ReadFile(templateFilepath)
	p.ensureNoError(err)
	tmpl, err := template.New(templateName).Parse(string(templateBytes))
	p.ensureNoError(err)
	var buff bytes.Buffer
	data := struct {
		*Config
		Namespace string
		// Deprecated: use Endpoint
		BrokerURL string
		Endpoint  interface{}
	}{
		p.config,
		p.client.Namespace,
		fmt.Sprintf("%v", endpoint),
		endpoint,
	}
	p.ensureNoError(tmpl.Execute(&buff, data))
	return buff.String()
}
