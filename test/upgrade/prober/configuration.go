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
	"path"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/wavesoftware/go-ensure"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/eventing/test/upgrade/prober/sut"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

const (
	defaultConfigName          = "wathola-config"
	defaultConfigHomedirPath   = ".config/wathola"
	defaultHomedir             = "/home/nonroot"
	defaultConfigFilename      = "config.toml"
	defaultWatholaEventsPrefix = "dev.knative.eventing.wathola"
	defaultHealthEndpoint      = "/healthz"
	defaultFinishedSleep       = 5 * time.Second

	Silence DuplicateAction = "silence"
	Warn    DuplicateAction = "warn"
	Error   DuplicateAction = "error"
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
	// Deprecated: use Wathola.SystemUnderTest instead.
	BrokerOpts []resources.BrokerOption
	// Namespace holds namespace in which test is about to be executed.
	// Deprecated: namespace is about to be taken from testlib.Client created by
	// CreateRunner
	Namespace string
}

// Wathola represents options related strictly to wathola testing tool.
type Wathola struct {
	ConfigToml
	SystemUnderTest  func(namespace string) sut.SystemUnderTest
	EventsTypePrefix string
	HealthEndpoint   string
}

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

// NewConfig creates a new configuration object with default values filled in.
// Values can be influenced by kelseyhightower/envconfig with
// `eventing_upgrade_tests` prefix.
func NewConfig(namespace ...string) *Config {
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
			ConfigToml: ConfigToml{
				ConfigTemplate:   defaultConfigFilename,
				ConfigMapName:    defaultConfigName,
				ConfigMountPoint: fmt.Sprintf("%s/%s", defaultHomedir, defaultConfigHomedirPath),
				ConfigFilename:   defaultConfigFilename,
			},
			EventsTypePrefix: defaultWatholaEventsPrefix,
			HealthEndpoint:   defaultHealthEndpoint,
			SystemUnderTest:  sut.NewDefault,
		},
	}

	err := envconfig.Process("eventing_upgrade_tests", config)
	ensure.NoError(err)
	if len(namespace) > 0 {
		config.Namespace = strings.Join(namespace, ",")
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
	s := p.config.SystemUnderTest(p.client.Namespace)
	url := s.Deploy(sc, dest)
	p.client.Cleanup(func() {
		s.Teardown(sc)
	})
	p.deployConfigToml(url)
}

func (p *prober) deployConfigToml(url *apis.URL) {
	name := p.config.ConfigMapName
	p.log.Infof("Deploying config map: \"%s/%s\"", p.client.Namespace, name)
	configData := p.compileTemplate(p.config.ConfigTemplate, url)
	p.client.CreateConfigMapOrFail(name, p.client.Namespace, map[string]string{
		p.config.ConfigFilename: configData,
	})
}

func (p *prober) compileTemplate(templateName string, brokerURL fmt.Stringer) string {
	_, filename, _, _ := runtime.Caller(0)
	templateFilepath := path.Join(path.Dir(filename), templateName)
	templateBytes, err := ioutil.ReadFile(templateFilepath)
	ensure.NoError(err)
	tmpl, err := template.New(templateName).Parse(string(templateBytes))
	ensure.NoError(err)
	var buff bytes.Buffer
	data := struct {
		*Config
		Namespace string
		BrokerURL string
	}{
		p.config,
		p.client.Namespace,
		brokerURL.String(),
	}
	ensure.NoError(tmpl.Execute(&buff, data))
	return buff.String()
}
