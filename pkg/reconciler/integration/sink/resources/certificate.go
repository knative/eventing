/*
Copyright 2024 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resources

import (
	"fmt"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/sinks/v1alpha1"
)

func MakeCertificate(sink *v1alpha1.IntegrationSink) *cmv1.Certificate {
	return &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName(sink) + "-cert",
			Namespace: sink.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(sink),
			},
		},
		Spec: cmv1.CertificateSpec{
			SecretName: DeploymentName(sink) + "-cert",
			SecretTemplate: &cmv1.CertificateSecretTemplate{
				Labels: map[string]string{
					"app.kubernetes.io/component": "knative-eventing",
					"app.kubernetes.io/name":      "integration-sink",
				},
			},
			Duration: &metav1.Duration{
				Duration: 90 * 24 * time.Hour, // 90 days
			},
			RenewBefore: &metav1.Duration{
				Duration: 15 * 24 * time.Hour, // 15 days
			},

			Subject: &cmv1.X509Subject{
				Organizations: []string{"local"},
			},
			PrivateKey: &cmv1.CertificatePrivateKey{
				Algorithm:      cmv1.RSAKeyAlgorithm,
				Encoding:       cmv1.PKCS1,
				Size:           2048,
				RotationPolicy: cmv1.RotationPolicyAlways,
			},
			DNSNames: []string{
				fmt.Sprintf("%s.%s.svc.cluster.local", DeploymentName(sink), sink.Namespace),
				fmt.Sprintf("%s.%s.svc", DeploymentName(sink), sink.Namespace),
			},
			IssuerRef: cmmeta.ObjectReference{
				Name:  "knative-eventing-ca-issuer",
				Kind:  "ClusterIssuer",
				Group: "cert-manager.io",
			},
		},
	}
}