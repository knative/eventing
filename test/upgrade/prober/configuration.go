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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"
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
		"knative-eventing-injection": "enabled",
	}
	_, err = p.client.Kube.Kube.CoreV1().Namespaces().
		Update(ns)
	ensure.NoError(err)
}

func (p *prober) deployConfigMap() {
	name := configName
	p.log.Infof("Deploying config map: %v", name)

	configData := p.compileTemplate(configFilename)
	data := make(map[string]string, 0)
	data[configFilename] = configData
	secret := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: data,
	}
	_, err := p.client.Kube.Kube.CoreV1().ConfigMaps(p.config.Namespace).
		Create(secret)
	ensure.NoError(err)
}

func (p *prober) deployTriggers() {
	for _, eventType := range eventTypes {
		name := fmt.Sprintf("wathola-trigger-%v", eventType)
		fullType := fmt.Sprintf("%v.%v", watholaEventNs, eventType)
		subscriberOption := resources.WithSubscriberServiceRefForTrigger(receiverName)
		if p.config.Serving.Use {
			subscriberOption = resources.WithSubscriberKServiceRefForTrigger(forwarderName)
		}
		trigger := resources.Trigger(
			name,
			resources.WithBroker("default"),
			resources.WithAttributesTriggerFilter(
				v1alpha1.TriggerAnyFilter,
				fullType,
				map[string]interface{}{},
			),
			subscriberOption,
		)
		triggers := p.client.Eventing.EventingV1alpha1().Triggers(p.config.Namespace)
		p.log.Infof("Deploying trigger: %v", name)
		// update trigger with the new reference
		_, err := triggers.Create(trigger)
		ensure.NoError(err)
		lib.WaitFor(fmt.Sprintf("trigger be ready: %v", name), func() error {
			meta := resources.NewMetaResource(name, p.config.Namespace, lib.TriggerTypeMeta)
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
		err := p.client.Eventing.EventingV1alpha1().Triggers(p.config.Namespace).
			Delete(name, &metav1.DeleteOptions{})
		ensure.NoError(err)
	}
}

func (p *prober) compileTemplate(templateName string) string {
	_, filename, _, _ := runtime.Caller(0)
	templateFilepath := path.Join(path.Dir(filename), templateName)
	templateBytes, err := ioutil.ReadFile(templateFilepath)
	ensure.NoError(err)
	tmpl, err := template.New(templateName).Parse(string(templateBytes))
	ensure.NoError(err)
	var buff bytes.Buffer
	ensure.NoError(tmpl.Execute(&buff, p.config))
	return buff.String()
}
