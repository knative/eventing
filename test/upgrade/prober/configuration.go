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

	"knative.dev/eventing/pkg/reconciler/sugar"

	"github.com/wavesoftware/go-ensure"
	corev1 "k8s.io/api/core/v1"
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
	p.annotateNamespace()
	p.deployConfigMap()
	p.deployTriggers()
}

func (p *prober) annotateNamespace() {
	ns, err := p.client.Kube.Kube.CoreV1().Namespaces().
		Get(p.client.Namespace, metav1.GetOptions{})
	ensure.NoError(err)
	ns.Labels = map[string]string{
		sugar.DeprecatedInjectionLabelKey: sugar.InjectionEnabledLabelValue,
		sugar.InjectionLabelKey:           sugar.InjectionEnabledLabelValue,
	}
	_, err = p.client.Kube.Kube.CoreV1().Namespaces().
		Update(ns)
	ensure.NoError(err)
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
	testlib.WaitFor(fmt.Sprintf("configmap be deployed: %v", name), func() error {
		p.log.Infof("Deploying config map: %v", name)

		brokerUrl, err := p.fetchBrokerUrl()
		if err != nil {
			return err
		}
		configData := p.compileTemplate(configFilename, brokerUrl)
		watholaConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Data: map[string]string{
				configFilename: configData,
			},
		}
		_, err = p.client.Kube.Kube.CoreV1().ConfigMaps(p.config.Namespace).Create(watholaConfigMap)
		if err != nil {
			return err
		}
		return nil
	})
}

func (p *prober) deployTriggers() {
	for _, eventType := range eventTypes {
		name := fmt.Sprintf("wathola-trigger-%v", eventType)
		fullType := fmt.Sprintf("%v.%v", watholaEventNs, eventType)
		subscriberOption := resources.WithSubscriberServiceRefForTriggerV1Beta1(receiverName)
		if p.config.Serving.Use {
			subscriberOption = resources.WithSubscriberKServiceRefForTrigger(forwarderName)
		}
		trigger := resources.TriggerV1Beta1(
			name,
			resources.WithBrokerV1Beta1(brokerName),
			resources.WithAttributesTriggerFilterV1Beta1(
				eventingv1beta1.TriggerAnyFilter,
				fullType,
				map[string]interface{}{},
			),
			subscriberOption,
		)
		triggers := p.client.Eventing.EventingV1beta1().Triggers(p.config.Namespace)
		p.log.Infof("Deploying trigger: %v", name)
		// update trigger with the new reference
		_, err := triggers.Create(trigger)
		ensure.NoError(err)
		testlib.WaitFor(fmt.Sprintf("trigger be ready: %v", name), func() error {
			meta := resources.NewMetaResource(name, p.config.Namespace, testlib.TriggerTypeMeta)
			return duck.WaitForResourceReady(p.client.Dynamic, meta)
		})
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
