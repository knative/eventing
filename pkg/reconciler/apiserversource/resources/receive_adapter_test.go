/*
Copyright 2020 The Knative Authors

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
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/ptr"

	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/pkg/reconciler/source"

	_ "knative.dev/pkg/metrics/testing"
	_ "knative.dev/pkg/system/testing"
)

func TestMakeReceiveAdapters(t *testing.T) {
	name := "source-name"
	one := int32(1)
	trueValue := true
	testCert := `
-----BEGIN CERTIFICATE-----
MIIDmjCCAoKgAwIBAgIUYzA4bTMXevuk3pl2Mn8hpCYL2C0wDQYJKoZIhvcNAQEL
BQAwLzELMAkGA1UEBhMCVVMxIDAeBgNVBAMMF0tuYXRpdmUtRXhhbXBsZS1Sb290
LUNBMB4XDTIzMDQwNTEzMTUyNFoXDTI2MDEyMzEzMTUyNFowbTELMAkGA1UEBhMC
VVMxEjAQBgNVBAgMCVlvdXJTdGF0ZTERMA8GA1UEBwwIWW91ckNpdHkxHTAbBgNV
BAoMFEV4YW1wbGUtQ2VydGlmaWNhdGVzMRgwFgYDVQQDDA9sb2NhbGhvc3QubG9j
YWwwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC5teo+En6U5nhqn7Sc
uanqswUmPlgs9j/8l21Rhb4T+ezlYKGQGhbJyFFMuiCE1Rjn8bpCwi7Nnv12Y2nz
FhEv2Jx0yL3Tqx0Q593myqKDq7326EtbO7wmDT0XD03twH5i9XZ0L0ihPWn1mjUy
WxhnHhoFpXrsnQECJorZY6aTrFbGVYelIaj5AriwiqyL0fET8pueI2GwLjgWHFSH
X8XsGAlcLUhkQG0Z+VO9usy4M1Wpt+cL6cnTiQ+sRmZ6uvaj8fKOT1Slk/oUeAi4
WqFkChGzGzLik0QrhKGTdw3uUvI1F2sdQj0GYzXaWqRz+tP9qnXdzk1GrszKKSlm
WBTLAgMBAAGjcDBuMB8GA1UdIwQYMBaAFJJcCftus4vj98N0zQQautsjEu82MAkG
A1UdEwQCMAAwCwYDVR0PBAQDAgTwMBQGA1UdEQQNMAuCCWxvY2FsaG9zdDAdBgNV
HQ4EFgQUnu/3vqA3VEzm128x/hLyZzR9JlgwDQYJKoZIhvcNAQELBQADggEBAFc+
1cKt/CNjHXUsirgEhry2Mm96R6Yxuq//mP2+SEjdab+FaXPZkjHx118u3PPX5uTh
gTT7rMfka6J5xzzQNqJbRMgNpdEFH1bbc11aYuhi0khOAe0cpQDtktyuDJQMMv3/
3wu6rLr6fmENo0gdcyUY9EiYrglWGtdXhlo4ySRY8UZkUScG2upvyOhHTxVCAjhP
efbMkNjmDuZOMK+wqanqr5YV6zMPzkQK7DspfRgasMAQmugQu7r2MZpXg8Ilhro1
s/wImGnMVk5RzpBVrq2VB9SkX/ThTVYEC/Sd9BQM364MCR+TA1l8/ptaLFLuwyw8
O2dgzikq8iSy1BlRsVw=
-----END CERTIFICATE-----
`

	src := &v1.ApiServerSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "source-namespace",
			UID:       "1234",
		},
		Spec: v1.ApiServerSourceSpec{
			Resources: []v1.APIVersionKindSelector{{
				APIVersion: "",
				Kind:       "Namespace",
			}, {
				APIVersion: "batch/v1",
				Kind:       "Job",
			}, {
				APIVersion: "",
				Kind:       "Pod",
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"test-key1": "test-value1"},
				},
			}},
			ResourceOwner: &v1.APIVersionKind{
				APIVersion: "custom/v1",
				Kind:       "Parent",
			},
			EventMode:          "Resource",
			ServiceAccountName: "source-svc-acct",
		},
	}

	want := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "source-namespace",
			Name:      kmeta.ChildName(fmt.Sprintf("apiserversource-%s-", name), string(src.UID)),
			Labels: map[string]string{
				"test-key1": "test-value1",
				"test-key2": "test-value2",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "sources.knative.dev/v1",
					Kind:               "ApiServerSource",
					Name:               name,
					UID:                "1234",
					Controller:         &trueValue,
					BlockOwnerDeletion: &trueValue,
				},
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test-key1": "test-value1",
					"test-key2": "test-value2",
				},
			},
			Replicas: &one,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "true",
					},
					Labels: map[string]string{
						"test-key1": "test-value1",
						"test-key2": "test-value2",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "source-svc-acct",
					EnableServiceLinks: ptr.Bool(false),
					Containers: []corev1.Container{
						{
							Name:  "receive-adapter",
							Image: "test-image",
							Ports: []corev1.ContainerPort{{
								Name:          "metrics",
								ContainerPort: 9090,
							}, {
								Name:          "health",
								ContainerPort: 8080,
							}},
							Env: []corev1.EnvVar{
								{
									Name:  "K_SINK",
									Value: "sink-uri",
								}, {
									Name:  "K_SOURCE_CONFIG",
									Value: `{"namespaces":["source-namespace"],"allNamespaces":false,"resources":[{"gvr":{"Group":"","Version":"","Resource":"namespaces"}},{"gvr":{"Group":"batch","Version":"v1","Resource":"jobs"}},{"gvr":{"Group":"","Version":"","Resource":"pods"},"selector":"test-key1=test-value1"}],"owner":{"apiVersion":"custom/v1","kind":"Parent"},"mode":"Resource"}`,
								}, {
									Name:  "SYSTEM_NAMESPACE",
									Value: "knative-testing",
								}, {
									Name: "NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								}, {
									Name:  "NAME",
									Value: name,
								}, {
									Name:  "METRICS_DOMAIN",
									Value: "knative.dev/eventing",
								}, {
									Name:  "K_CA_CERTS",
									Value: testCert,
								}, {
									Name:  source.EnvLoggingCfg,
									Value: "",
								}, {
									Name:  source.EnvMetricsCfg,
									Value: "",
								}, {
									Name:  source.EnvTracingCfg,
									Value: "",
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Port: intstr.FromString("health"),
									},
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.Bool(false),
								ReadOnlyRootFilesystem:   ptr.Bool(true),
								RunAsNonRoot:             ptr.Bool(true),
								Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
								SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
							},
						},
					},
				},
			},
		},
	}

	ceSrc := src.DeepCopy()
	ceSrc.Spec.CloudEventOverrides = &duckv1.CloudEventOverrides{Extensions: map[string]string{"1": "one"}}
	ceWant := want.DeepCopy()
	ceWant.Spec.Template.Spec.Containers[0].Env = append(ceWant.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
		Name:  "K_CE_OVERRIDES",
		Value: `{"extensions":{"1":"one"}}`,
	})

	testCases := map[string]struct {
		want *appsv1.Deployment
		src  *v1.ApiServerSource
	}{
		"TestMakeReceiveAdapter": {

			want: want,
			src:  src,
		}, "TestMakeReceiveAdapterWithExtensionOverride": {
			src:  ceSrc,
			want: ceWant,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {

			got, _ := MakeReceiveAdapter(&ReceiveAdapterArgs{
				Image:  "test-image",
				Source: tc.src,
				Labels: map[string]string{
					"test-key1": "test-value1",
					"test-key2": "test-value2",
				},
				SinkURI:    "sink-uri",
				CACerts:    &testCert,
				Configs:    &source.EmptyVarsGenerator{},
				Namespaces: []string{"source-namespace"},
			})

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Error("unexpected deploy (-want, +got) =", diff)
			}

		})
	}
}
