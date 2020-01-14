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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/test/common"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"path"
	"runtime"
	"text/template"
)

const (
	configName     = "wathola-config"
	configFilename = "config.toml"
	watholaEventNs = "com.github.cardil.wathola"
)

var (
	eventTypes = []string{"step", "finished"}
)

func (p *prober) deployConfiguration() {
	p.deploySecret()
	p.deployTriggers()
}

func (p *prober) deploySecret() {
	name := configName
	p.logf("Deploying secret: %v", name)

	configData := p.compileTemplate(configFilename)
	data := make(map[string]string, 0)
	data[configFilename] = configData
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		StringData: data,
		Type:       "Opaque",
	}
	_, err := p.client.Kube.Kube.CoreV1().Secrets(p.config.Namespace).
		Create(secret)
	common.NoError(err)
}

func (p *prober) deployTriggers() {
	for _, eventType := range eventTypes {
		name := fmt.Sprintf("wathola-trigger-%v", eventType)
		fullType := fmt.Sprintf("%v.%v", watholaEventNs, eventType)
		ref := &corev1.ObjectReference{
			Kind:       "Service",
			Namespace:  p.config.Namespace,
			Name:       receiverName,
			APIVersion: "v1",
		}
		if p.config.Serving.Use {
			ref.APIVersion = servicesCR.GroupVersion().String()
			ref.Name = forwarderName
		}
		trigger := &v1alpha1.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: v1alpha1.TriggerSpec{
				Broker: "default",
				Filter: &v1alpha1.TriggerFilter{
					Attributes: &v1alpha1.TriggerFilterAttributes{
						"type": fullType,
					},
				},
				Subscriber: duckv1.Destination{
					Ref: ref,
				},
			},
		}
		p.logf("Deploying trigger: %v", name)
		_, err := p.client.Eventing.EventingV1alpha1().Triggers(p.config.Namespace).
			Create(trigger)
		common.NoError(err)
	}
}

func (p *prober) removeConfiguration() {
	p.removeSecret()
	p.removeTriggers()
}

func (p *prober) removeSecret() {
	p.logf("Removing secret: %v", configName)
	err := p.client.Kube.Kube.CoreV1().Secrets(p.config.Namespace).
		Delete(configName, &metav1.DeleteOptions{})
	common.NoError(err)
}

func (p *prober) removeTriggers() {
	for _, eventType := range eventTypes {
		name := fmt.Sprintf("wathola-trigger-%v", eventType)
		p.logf("Removing trigger: %v", name)
		err := p.client.Eventing.EventingV1alpha1().Triggers(p.config.Namespace).
			Delete(name, &metav1.DeleteOptions{})
		common.NoError(err)
	}
}

func (p *prober) compileTemplate(templateName string) string {
	_, filename, _, _ := runtime.Caller(0)
	templateFilepath := path.Join(path.Dir(filename), templateName)
	templateBytes, err := ioutil.ReadFile(templateFilepath)
	common.NoError(err)
	tmpl, err := template.New(templateName).Parse(string(templateBytes))
	common.NoError(err)
	var buff bytes.Buffer
	common.NoError(tmpl.Execute(&buff, p.config))
	return buff.String()
}
