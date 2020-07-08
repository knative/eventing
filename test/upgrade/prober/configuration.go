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
	"fmt"
	"io/ioutil"
	"path"
	"runtime"
	"text/template"

	"github.com/wavesoftware/go-ensure"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/pkg/apis"
)

const (
	configName       = "wathola-config"
	configMountPoint = "/home/nonroot/.config/wathola"
	configFilename   = "config.toml"
	watholaEventNs   = "com.github.cardil.wathola"
	healthEndpoint   = "/healthz"
)

var (
	eventTypes = []string{"step", "finished"}
	brokerName = "default"
)

func (p *prober) deployConfiguration() {
	p.deployBroker()
	p.deployConfigMap()
	p.deployTriggers()
}

func (p *prober) deployBroker() {
	p.client.CreateBrokerV1Beta1OrFail(brokerName)
}

func (p *prober) fetchBrokerUrl() (*apis.URL, error) {
	namespace := p.config.Namespace
	p.log.Debugf("Fetching %s broker URL for ns %s", brokerName, namespace)
	meta := resources.NewMetaResource(brokerName, p.config.Namespace, testlib.BrokerTypeMeta)
	err := duck.WaitForResourceReady(p.client.Dynamic, meta)
	if err != nil {
		return nil, err
	}
	broker, err := p.client.Eventing.EventingV1beta1().Brokers(namespace).Get(brokerName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	url := broker.Status.Address.URL
	p.log.Debugf("%s broker URL for ns %s is %v", brokerName, namespace, url)
	return url, nil
}

func (p *prober) deployConfigMap() {
	name := configName
	p.log.Infof("Deploying config map: \"%s/%s\"", p.config.Namespace, name)
	brokerUrl, err := p.fetchBrokerUrl()
	ensure.NoError(err)
	configData := p.compileTemplate(configFilename, brokerUrl)
	p.client.CreateConfigMapOrFail(name, p.config.Namespace, map[string]string{configFilename: configData})
}

func (p *prober) deployTriggers() {
	for _, eventType := range eventTypes {
		name := fmt.Sprintf("wathola-trigger-%v", eventType)
		fullType := fmt.Sprintf("%v.%v", watholaEventNs, eventType)
		subscriberOption := resources.WithSubscriberServiceRefForTriggerV1Beta1(receiverName)
		if p.config.Serving.Use {
			subscriberOption = resources.WithSubscriberKServiceRefForTrigger(forwarderName)
		}
		p.client.CreateTriggerOrFailV1Beta1(name,
			resources.WithBrokerV1Beta1(brokerName),
			resources.WithAttributesTriggerFilterV1Beta1(
				eventingv1beta1.TriggerAnyFilter,
				fullType,
				map[string]interface{}{},
			),
			subscriberOption)
	}
}

func (p *prober) removeConfiguration() {
	p.removeConfigMap()
	p.removeTriggers()
}

func (p *prober) removeConfigMap() {
	p.log.Infof("Removing config map: %v", configName)
	err := p.client.Kube.Kube.CoreV1().ConfigMaps(p.config.Namespace).
		Delete(configName, &metav1.DeleteOptions{})
	ensure.NoError(err)
}

func (p *prober) removeTriggers() {
	for _, eventType := range eventTypes {
		name := fmt.Sprintf("wathola-trigger-%v", eventType)
		p.log.Infof("Removing trigger: %v", name)
		err := p.client.Eventing.EventingV1beta1().Triggers(p.config.Namespace).
			Delete(name, &metav1.DeleteOptions{})
		ensure.NoError(err)
	}
}

func (p *prober) compileTemplate(templateName string, brokerUrl *apis.URL) string {
	_, filename, _, _ := runtime.Caller(0)
	templateFilepath := path.Join(path.Dir(filename), templateName)
	templateBytes, err := ioutil.ReadFile(templateFilepath)
	ensure.NoError(err)
	tmpl, err := template.New(templateName).Parse(string(templateBytes))
	ensure.NoError(err)
	var buff bytes.Buffer
	data := struct {
		*Config
		BrokerUrl string
	}{
		p.config,
		brokerUrl.String(),
	}
	ensure.NoError(tmpl.Execute(&buff, data))
	return buff.String()
}
