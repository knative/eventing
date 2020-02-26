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
	"fmt"

	"github.com/wavesoftware/go-ensure"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"knative.dev/pkg/test"
)

var (
	receiverName           = "wathola-receiver"
	receiverNodePort int32 = -1
)

func (p *prober) deployReceiver() {
	p.deployReceiverPod()
	p.deployReceiverService()
}

func (p *prober) removeReceiver() {
	p.removeReceiverService()
	p.removeReceiverPod()
}

func (p *prober) deployReceiverPod() {
	p.log.Infof("Deploy of receiver pod: %v", receiverName)
	pod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      receiverName,
			Namespace: p.config.Namespace,
			Labels: map[string]string{
				"app": receiverName,
			},
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: configName,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: configName,
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "receiver",
					Image: fmt.Sprintf("quay.io/cardil/wathola-receiver:%v", Version),
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      configName,
							ReadOnly:  true,
							MountPath: configMountPoint,
						},
					},
					ReadinessProbe: &corev1.Probe{
						Handler: corev1.Handler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: healthEndpoint,
								Port: intstr.FromInt(22111),
							},
						},
					},
				},
			},
		},
	}
	_, err := p.client.Kube.CreatePod(pod)
	ensure.NoError(err)

	waitFor(fmt.Sprintf("receiver be ready: %v", receiverName), func() error {
		return test.WaitForPodRunning(p.client.Kube, receiverName, p.client.Namespace)
	})
}

func (p *prober) deployReceiverService() {
	p.log.Infof("Deploy of receiver service: %v", receiverName)
	service := &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name:      receiverName,
			Namespace: p.config.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     "http",
					Protocol: "TCP",
					Port:     80,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 22111,
					},
				},
			},
			Selector: map[string]string{
				"app": receiverName,
			},
			Type: corev1.ServiceTypeNodePort,
		},
	}
	created, err := p.client.Kube.Kube.CoreV1().Services(p.config.Namespace).
		Create(service)
	ensure.NoError(err)
	for _, portSpec := range created.Spec.Ports {
		if portSpec.Port == 80 {
			receiverNodePort = portSpec.NodePort
		}
	}
	if receiverNodePort == -1 {
		panic(fmt.Errorf("couldn't find a node port for service: %v", receiverName))
	} else {
		p.log.Debugf("Node port for service: %v is %v", receiverName, receiverNodePort)
	}
}

func (p *prober) removeReceiverPod() {
	p.log.Infof("Remove of receiver pod: %v", receiverName)
	err := p.client.Kube.Kube.CoreV1().Pods(p.config.Namespace).
		Delete(receiverName, &v1.DeleteOptions{})
	ensure.NoError(err)
}

func (p *prober) removeReceiverService() {
	p.log.Infof("Remove of receiver service: %v", receiverName)
	err := p.client.Kube.Kube.CoreV1().Services(p.config.Namespace).
		Delete(receiverName, &v1.DeleteOptions{})
	ensure.NoError(err)
}
