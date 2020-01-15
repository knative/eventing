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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/test/common"
)

var senderName = "wathola-sender"

func (p *prober) deploySender() {
	p.log.Infof("Deploy sender pod: %v", senderName)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      senderName,
			Namespace: p.config.Namespace,
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
					Name:  "sender",
					Image: fmt.Sprintf("quay.io/cardil/wathola-sender:%v", Version),
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      configName,
							ReadOnly:  true,
							MountPath: configMountPoint,
						},
					},
				},
			},
		},
	}
	_, err := p.client.Kube.Kube.CoreV1().Pods(p.client.Namespace).
		Create(pod)
	common.NoError(err)

	waitFor(fmt.Sprintf("sender pod be ready: %v", senderName), func() error {
		return p.waitForPodReady(senderName, p.client.Namespace)
	})
}

func (p *prober) removeSender() {
	p.log.Infof("Remove of sender pod: %v", senderName)

	err := p.client.Kube.Kube.CoreV1().Pods(p.client.Namespace).
		Delete(senderName, &metav1.DeleteOptions{})
	common.NoError(err)
}