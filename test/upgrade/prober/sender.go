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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testlib "knative.dev/eventing/test/lib"
	pkgTest "knative.dev/pkg/test"
)

var senderName = "wathola-sender"

func (p *prober) deploySender(ctx context.Context) {
	p.log.Info("Deploy sender deployment: ", senderName)
	var replicas int32 = 1
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      senderName,
			Namespace: p.config.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": senderName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": senderName,
					},
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{{
						Name: p.config.ConfigMapName,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: p.config.ConfigMapName,
								},
							},
						},
					}},
					Containers: []corev1.Container{{
						Name:  "sender",
						Image: pkgTest.ImagePath(senderName),
						VolumeMounts: []corev1.VolumeMount{{
							Name:      p.config.ConfigMapName,
							ReadOnly:  true,
							MountPath: p.config.ConfigMountPoint,
						}},
					}},
				},
			},
		},
	}

	_, err := p.client.Kube.AppsV1().
		Deployments(p.client.Namespace).
		Create(ctx, deployment, metav1.CreateOptions{})
	ensure.NoError(err)

	testlib.WaitFor(fmt.Sprint("sender deployment be ready: ", senderName), func() error {
		return pkgTest.WaitForDeploymentScale(
			ctx, p.client.Kube, senderName, p.client.Namespace, int(replicas),
		)
	})
}

func (p *prober) removeSender(ctx context.Context) {
	p.log.Info("Remove of sender deployment: ", senderName)

	err := p.client.Kube.AppsV1().
		Deployments(p.client.Namespace).
		Delete(ctx, senderName, metav1.DeleteOptions{})
	ensure.NoError(err)
}
