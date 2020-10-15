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
	"context"
	"fmt"

	"github.com/wavesoftware/go-ensure"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testlib "knative.dev/eventing/test/lib"
	pkgTest "knative.dev/pkg/test"
)

var senderName = "wathola-sender"

func (p *prober) deploySender(ctx context.Context) {
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
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: configName},
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "sender",
					Image: pkgTest.ImagePath(senderName),
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
	_, err := p.client.Kube.CoreV1().Pods(p.client.Namespace).
		Create(ctx, pod, metav1.CreateOptions{})
	ensure.NoError(err)

	testlib.WaitFor(fmt.Sprintf("sender pod be ready: %v", senderName), func() error {
		return pkgTest.WaitForPodRunning(ctx, p.client.Kube, senderName, p.client.Namespace)
	})
}

func (p *prober) removeSender(ctx context.Context) {
	p.log.Infof("Remove of sender pod: %v", senderName)

	err := p.client.Kube.CoreV1().Pods(p.client.Namespace).
		Delete(ctx, senderName, metav1.DeleteOptions{})
	ensure.NoError(err)
}
