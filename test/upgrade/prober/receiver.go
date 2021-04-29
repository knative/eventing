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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	testlib "knative.dev/eventing/test/lib"
	watholaconfig "knative.dev/eventing/test/upgrade/prober/wathola/config"
	pkgTest "knative.dev/pkg/test"
)

var (
	receiverName           = "wathola-receiver"
	receiverNodePort int32 = -1
)

func (p *prober) deployReceiver() {
	p.deployReceiverDeployment()
	p.deployReceiverService()
}

func (p *prober) deployReceiverDeployment() {
	p.log.Info("Deploy of receiver deployment: ", receiverName)
	deployment := p.createReceiverDeployment()
	_, err := p.client.Kube.AppsV1().
		Deployments(deployment.Namespace).
		Create(p.client.Ctx, deployment, metav1.CreateOptions{})
	ensure.NoError(err)

	testlib.WaitFor(fmt.Sprint("receiver deployment be ready: ", receiverName), func() error {
		return pkgTest.WaitForDeploymentScale(
			p.client.Ctx, p.client.Kube, receiverName, p.client.Namespace, 1,
		)
	})
}

func (p *prober) deployReceiverService() {
	p.log.Infof("Deploy of receiver service: %v", receiverName)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      receiverName,
			Namespace: p.client.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     "http",
					Protocol: "TCP",
					Port:     80,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: watholaconfig.DefaultReceiverPort,
					},
				},
			},
			Selector: map[string]string{
				"app": receiverName,
			},
			Type: corev1.ServiceTypeNodePort,
		},
	}
	created, err := p.client.Kube.CoreV1().Services(p.client.Namespace).
		Create(p.client.Ctx, service, metav1.CreateOptions{})
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

func (p *prober) createReceiverDeployment() *appsv1.Deployment {
	var replicas int32 = 1
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      receiverName,
			Namespace: p.client.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": receiverName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": receiverName,
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
						Name:  "receiver",
						Image: pkgTest.ImagePath(receiverName),
						VolumeMounts: []corev1.VolumeMount{{
							Name:      p.config.ConfigMapName,
							ReadOnly:  true,
							MountPath: p.config.ConfigMountPoint,
						}},
						ReadinessProbe: &corev1.Probe{
							Handler: corev1.Handler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: p.config.HealthEndpoint,
									Port: intstr.FromInt(watholaconfig.DefaultReceiverPort),
								},
							},
						},
					}},
				},
			},
		},
	}
}
