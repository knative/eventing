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
	"text/template"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/wavesoftware/go-ensure"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/pkg/apis"
)

const (
	defaultConfigName          = "wathola-config"
	defaultConfigHomedirPath   = ".config/wathola"
	defaultHomedir             = "/home/nonroot"
	defaultConfigFilename      = "config.toml"
	defaultWatholaEventsPrefix = "com.github.cardil.wathola"
	defaultBrokerName          = "default"
	defaultHealthEndpoint      = "/healthz"
	defaultFinishedSleep       = 5 * time.Second

	Silence DuplicateAction = "silence"
	Warn    DuplicateAction = "warn"
	Error   DuplicateAction = "error"
)

// DuplicateAction is the action to take in case of duplicated events
type DuplicateAction string

var eventTypes = []string{"step", "finished"}

// Config represents a configuration for prober.
type Config struct {
	Wathola
	Namespace     string
	Interval      time.Duration
	FinishedSleep time.Duration
	Serving       ServingConfig
	FailOnErrors  bool
	OnDuplicate   DuplicateAction
	BrokerOpts    []resources.BrokerOption
}

// Wathola represents options related strictly to wathola testing tool.
type Wathola struct {
	ConfigMap
	EventsTypePrefix string
	HealthEndpoint   string
	BrokerName       string
}

// ConfigMap represents options of wathola config toml file.
type ConfigMap struct {
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
// `e2e_upgrade_tests` prefix.
func NewConfig(namespace string) *Config {
	config := &Config{
		Namespace:     "",
		Interval:      Interval,
		FinishedSleep: defaultFinishedSleep,
		FailOnErrors:  true,
		OnDuplicate:   Warn,
		BrokerOpts:    make([]resources.BrokerOption, 0),
		Serving: ServingConfig{
			Use:         false,
			ScaleToZero: true,
		},
		Wathola: Wathola{
			ConfigMap: ConfigMap{
				ConfigTemplate:   defaultConfigFilename,
				ConfigMapName:    defaultConfigName,
				ConfigMountPoint: fmt.Sprintf("%s/%s", defaultHomedir, defaultConfigHomedirPath),
				ConfigFilename:   defaultConfigFilename,
			},
			EventsTypePrefix: defaultWatholaEventsPrefix,
			HealthEndpoint:   defaultHealthEndpoint,
			BrokerName:       defaultBrokerName,
		},
	}

	// FIXME: remove while fixing https://github.com/knative/eventing/issues/2665
	config.FailOnErrors = false

	err := envconfig.Process("e2e_upgrade_tests", config)
	ensure.NoError(err)
	config.Namespace = namespace
	return config
}

func (p *prober) deployConfiguration() {
	p.deployBroker()
	p.deployConfigMap()
	p.deployTriggers()
}

func (p *prober) deployBroker() {
	p.client.CreateBrokerOrFail(p.config.BrokerName, p.config.BrokerOpts...)
}

func (p *prober) fetchBrokerURL() (*apis.URL, error) {
	namespace := p.config.Namespace
	p.log.Debugf("Fetching %s broker URL for ns %s",
		p.config.BrokerName, namespace)
	meta := resources.NewMetaResource(
		p.config.BrokerName, p.config.Namespace, testlib.BrokerTypeMeta,
	)
	err := duck.WaitForResourceReady(p.client.Dynamic, meta)
	if err != nil {
		return nil, err
	}
	broker, err := p.client.Eventing.EventingV1().Brokers(namespace).Get(
		context.Background(), p.config.BrokerName, metav1.GetOptions{},
	)
	if err != nil {
		return nil, err
	}
	url := broker.Status.Address.URL
	p.log.Debugf("%s broker URL for ns %s is %v",
		p.config.BrokerName, namespace, url)
	return url, nil
}

func (p *prober) deployConfigMap() {
	name := p.config.ConfigMapName
	p.log.Infof("Deploying config map: \"%s/%s\"", p.config.Namespace, name)
	brokerURL, err := p.fetchBrokerURL()
	ensure.NoError(err)
	configData := p.compileTemplate(p.config.ConfigTemplate, brokerURL)
	p.client.CreateConfigMapOrFail(name, p.config.Namespace, map[string]string{
		p.config.ConfigFilename: configData,
	})
}

func (p *prober) deployTriggers() {
	for _, eventType := range eventTypes {
		name := fmt.Sprintf("wathola-trigger-%v", eventType)
		fullType := fmt.Sprintf("%v.%v", p.config.EventsTypePrefix, eventType)
		subscriberOption := resources.WithSubscriberServiceRefForTrigger(receiverName)
		if p.config.Serving.Use {
			subscriberOption = resources.WithSubscriberKServiceRefForTrigger(forwarderName)
		}
		p.client.CreateTriggerOrFail(name,
			resources.WithBroker(p.config.BrokerName),
			resources.WithAttributesTriggerFilter(
				eventingv1.TriggerAnyFilter,
				fullType,
				map[string]interface{}{},
			),
			subscriberOption,
		)
	}
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
		BrokerURL string
	}{
		p.config,
		brokerURL.String(),
	}
	ensure.NoError(tmpl.Execute(&buff, data))
	return buff.String()
}
