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

package v1

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	configmapinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/fake"
	fakedynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/tracker"

	. "knative.dev/eventing/pkg/scheduler/testing"
)

var (
	caCert = `
	-----BEGIN CERTIFICATE-----
	MIIDPzCCAiegAwIBAgIUYuysnNGPwBjbiDRc+/9s9Jl3N8YwDQYJKoZIhvcNAQEL
	BQAwLzELMAkGA1UEBhMCVVMxIDAeBgNVBAMMF0tuYXRpdmUtRXhhbXBsZS1Sb290
	LUNBMB4XDTIzMDQwNTEzMTQxMloXDTI2MDEyMzEzMTQxMlowLzELMAkGA1UEBhMC
	VVMxIDAeBgNVBAMMF0tuYXRpdmUtRXhhbXBsZS1Sb290LUNBMIIBIjANBgkqhkiG
	9w0BAQEFAAOCAQ8AMIIBCgKCAQEAyEwyvWKc/SJzblAc/pNIE7UJHIpbEUDtwOom
	YvytwcMhI73zlSVhAcOagwnn3AvBg3McGPyLGghr9EuXBE1Vx584Pw1cmKOwbyiC
	SQtaRwbztzM555T4Rtrk4tdKm+WHD/HiYAB/s+OnPJ6F6yBedT6nW08HlTP5lJX1
	U21+OAiOSU4zx+YYlkRbHq8aYggB1YM+hdRSStl9Mc/nw6TWlVsd2LjppXgoxSKl
	YTB4ZwnaKmrIRa9hFf1DVY/nTlmUP2iGr9131CLs3/5QyoFRWI6ayfnRSkmVwKLS
	8AW/b4jh+qJVIaeLCw5QF4RuqsE5VaUj6wlEqWM4eI+5Uaj+5QIDAQABo1MwUTAd
	BgNVHQ4EFgQUklwJ+26zi+P3w3TNBBq62yMS7zYwHwYDVR0jBBgwFoAUklwJ+26z
	i+P3w3TNBBq62yMS7zYwDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOC
	AQEAWwlatEXUTiB4O3M/fLSZ4JlAA1bq2U+dafiUiq5Ym0F1/UGu7YD74LGm4n03
	X9QU4jVwAkxL8pFV68NEBFJXOwFRyVQ1THAfhzij5teMAd4aqaffEPF0YfE8+rdg
	MSQx9n/OOeeyqWlaAqI3D9SEoSFPk5Xbfdzu6zGggizJwIYus77LOYxS7hvGxCci
	dTnEHvGoP14/13F/2vZLSaH9qrAv3cTenVYRN1QSSVI0V2XAhz+HAOjO2muaaYEG
	2eKiYvHvG0p5aCRIZYi4z3q6QAr9z+nyRyO1Tw/CnbCOeULQoOZWLy8xE9zBOE1t
	JQArXobwA4IZrx13xxsMafyt0A==
	-----END CERTIFICATE-----
	`
)

func init() {
	duckv1.AddToScheme(scheme.Scheme)
}

func TestSinkBindingGetConditionSet(t *testing.T) {
	r := &SinkBinding{}

	if got, want := r.GetConditionSet().GetTopLevelConditionType(), apis.ConditionReady; got != want {
		t.Errorf("GetTopLevelCondition=%v, want=%v", got, want)
	}
}

func TestSinkBindingGetGroupVersionKind(t *testing.T) {
	r := &SinkBinding{}
	want := schema.GroupVersionKind{
		Group:   "sources.knative.dev",
		Version: "v1",
		Kind:    "SinkBinding",
	}
	if got := r.GetGroupVersionKind(); got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestSinkBindingGetters(t *testing.T) {
	r := &SinkBinding{
		Spec: SinkBindingSpec{
			BindingSpec: duckv1.BindingSpec{
				Subject: tracker.Reference{
					APIVersion: "foo",
				},
			},
		},
	}
	if got, want := r.GetUntypedSpec(), r.Spec; !reflect.DeepEqual(got, want) {
		t.Errorf("GetUntypedSpec() = %v, want: %v", got, want)
	}
	if got, want := r.GetSubject(), r.Spec.Subject; !reflect.DeepEqual(got, want) {
		t.Errorf("GetSubject() = %v, want: %v", got, want)
	}
	if got, want := r.GetBindingStatus(), &r.Status; !reflect.DeepEqual(got, want) {
		t.Errorf("GetBindingStatus() = %v, want: %v", got, want)
	}
}

func TestSinkBindingSetObsGen(t *testing.T) {
	r := &SinkBinding{
		Spec: SinkBindingSpec{
			BindingSpec: duckv1.BindingSpec{
				Subject: tracker.Reference{
					APIVersion: "foo",
				},
			},
		},
	}
	want := int64(3762)
	r.GetBindingStatus().SetObservedGeneration(want)
	if got := r.Status.ObservedGeneration; got != want {
		t.Errorf("SetObservedGeneration() = %d, wanted %d", got, want)
	}
}

func TestSinkBindingStatusIsReady(t *testing.T) {
	sink := &duckv1.Addressable{
		Name:    pointer.String("http"),
		URL:     apis.HTTP("table.ns.svc.cluster.local/flip"),
		CACerts: &caCert,
	}
	sink.URL.Scheme = "uri"
	tests := []struct {
		name string
		s    *SinkBindingStatus
		want bool
	}{{
		name: "uninitialized",
		s:    &SinkBindingStatus{},
		want: false,
	}, {
		name: "initialized",
		s: func() *SinkBindingStatus {
			s := &SinkBindingStatus{}
			s.InitializeConditions()
			return s
		}(),
		want: false,
	}, {
		name: "mark binding unavailable",
		s: func() *SinkBindingStatus {
			s := &SinkBindingStatus{}
			s.InitializeConditions()
			s.MarkBindingUnavailable("TheReason", "this is the message")
			return s
		}(),
		want: false,
	}, {
		name: "mark sink",
		s: func() *SinkBindingStatus {
			s := &SinkBindingStatus{}
			s.InitializeConditions()
			s.MarkSink(sink)
			s.MarkBindingUnavailable("TheReason", "this is the message")
			return s
		}(),
		want: false,
	}, {
		name: "mark available",
		s: func() *SinkBindingStatus {
			s := &SinkBindingStatus{}
			s.InitializeConditions()
			s.MarkSink(sink)
			s.MarkBindingAvailable()
			s.MarkOIDCIdentityCreatedSucceeded()
			s.MarkOIDCTokenSecretCreatedSuccceeded()
			return s
		}(),
		want: true,
	}, {
		name: "mark OIDC identity created",
		s: func() *SinkBindingStatus {
			s := &SinkBindingStatus{}
			s.InitializeConditions()
			s.MarkSink(sink)
			s.MarkBindingAvailable()
			s.MarkOIDCIdentityCreatedSucceeded()
			s.MarkOIDCTokenSecretCreatedSuccceeded()
			return s
		}(),
		want: true,
	}, {
		name: "mark OIDC identity created with reason",
		s: func() *SinkBindingStatus {
			s := &SinkBindingStatus{}
			s.InitializeConditions()
			s.MarkSink(sink)
			s.MarkBindingAvailable()
			s.MarkOIDCIdentityCreatedSucceededWithReason("TheReason", "feature is disabled")
			s.MarkOIDCTokenSecretCreatedSuccceeded()
			return s
		}(),
		want: true,
	}, {
		name: "mark OIDC identity created failed",
		s: func() *SinkBindingStatus {
			s := &SinkBindingStatus{}
			s.InitializeConditions()
			s.MarkSink(sink)
			s.MarkBindingAvailable()
			s.MarkOIDCIdentityCreatedFailed("TheReason", "this is a message")
			s.MarkOIDCTokenSecretCreatedSuccceeded()
			return s
		}(),
		want: false,
	}, {
		name: "mark OIDC token secret created",
		s: func() *SinkBindingStatus {
			s := &SinkBindingStatus{}
			s.InitializeConditions()
			s.MarkSink(sink)
			s.MarkBindingAvailable()
			s.MarkOIDCIdentityCreatedSucceeded()
			s.MarkOIDCTokenSecretCreatedSuccceeded()
			return s
		}(),
		want: true,
	}, {
		name: "mark OIDC token secret failed",
		s: func() *SinkBindingStatus {
			s := &SinkBindingStatus{}
			s.InitializeConditions()
			s.MarkSink(sink)
			s.MarkBindingAvailable()
			s.MarkOIDCIdentityCreatedSucceeded()
			s.MarkOIDCTokenSecretCreatedFailed("Some", "reason")
			return s
		}(),
		want: false,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.s.IsReady()
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("%s: unexpected condition (-want, +got) = %v", test.name, diff)
			}
		})
	}
}

func TestSinkBindingUndo(t *testing.T) {
	tests := []struct {
		name string
		in   *duckv1.WithPod
		want *duckv1.WithPod
	}{{
		name: "nothing to remove",
		in: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
						}},
					},
				},
			},
		},
		want: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
						}},
					},
				},
			},
		},
	}, {
		name: "Remove trust bundle volumes",
		in: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "kne-bundle-knative-eventing-bundle" + strings.Repeat("a", 29),
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "knative-eventing-bundle" + strings.Repeat("a", 29),
										},
									},
								},
							},
						},
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "kne-bundle-knative-eventing-bundle" + strings.Repeat("a", 29),
									MountPath: "/knative-custom-certs/knative-eventing-bundle" + strings.Repeat("a", 29),
									ReadOnly:  true,
								},
							},
						}},
					},
				},
			},
		},
		want: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{},
						Containers: []corev1.Container{{
							Name:         "blah",
							Image:        "busybox",
							VolumeMounts: []corev1.VolumeMount{},
						}},
					},
				},
			},
		},
	}, {
		name: "lots to remove",
		in: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{{
							Name:  "setup",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "FOO",
								Value: "BAR",
							}, {
								Name:  "K_SINK",
								Value: "http://localhost:8080",
							}, {
								Name:  "K_CA_CERTS",
								Value: caCert,
							}, {
								Name:  "BAZ",
								Value: "INGA",
							}, {
								Name:  "K_CE_OVERRIDES",
								Value: `{"extensions":{"foo":"bar"}}`,
							}},
							VolumeMounts: []corev1.VolumeMount{{
								Name: "foo",
							}, {
								Name: oidcTokenVolumeName,
							}},
						}},
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "FOO",
								Value: "BAR",
							}, {
								Name:  "K_SINK",
								Value: "http://localhost:8080",
							}, {
								Name:  "K_CA_CERTS",
								Value: caCert,
							}, {
								Name:  "BAZ",
								Value: "INGA",
							}, {
								Name:  "K_CE_OVERRIDES",
								Value: `{"extensions":{"foo":"bar"}}`,
							}},
							VolumeMounts: []corev1.VolumeMount{{
								Name: "foo",
							}, {
								Name: oidcTokenVolumeName,
							}},
						}, {
							Name:  "sidecar",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "K_SINK",
								Value: "http://localhost:8080",
							}, {
								Name:  "K_CA_CERTS",
								Value: caCert,
							}, {
								Name:  "BAZ",
								Value: "INGA",
							}, {
								Name:  "K_CE_OVERRIDES",
								Value: `{"extensions":{"foo":"bar"}}`,
							}},
							VolumeMounts: []corev1.VolumeMount{{
								Name: "foo",
							}, {
								Name: oidcTokenVolumeName,
							}},
						}},
						Volumes: []corev1.Volume{{
							Name: "foo",
						}, {
							Name: oidcTokenVolumeName,
						}},
					},
				},
			},
		},
		want: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{{
							Name:  "setup",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "FOO",
								Value: "BAR",
							}, {
								Name:  "BAZ",
								Value: "INGA",
							}},
							VolumeMounts: []corev1.VolumeMount{{
								Name: "foo",
							}},
						}},
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "FOO",
								Value: "BAR",
							}, {
								Name:  "BAZ",
								Value: "INGA",
							}},
							VolumeMounts: []corev1.VolumeMount{{
								Name: "foo",
							}},
						}, {
							Name:  "sidecar",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "BAZ",
								Value: "INGA",
							}},
							VolumeMounts: []corev1.VolumeMount{{
								Name: "foo",
							}},
						}},
						Volumes: []corev1.Volume{{
							Name: "foo",
						}},
					},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.in
			sb := &SinkBinding{}
			sb.Undo(context.Background(), got)

			if !cmp.Equal(got, test.want) {
				t.Error("Undo (-want, +got):", cmp.Diff(test.want, got))
			}
		})
	}
}

func TestSinkBindingDo(t *testing.T) {
	destination := duckv1.Destination{
		URI: &apis.URL{
			Scheme: "http",
			Host:   "thing.ns.svc.cluster.local",
			Path:   "/a/path",
		},
		CACerts: &caCert,
	}

	overrides := duckv1.CloudEventOverrides{Extensions: map[string]string{"foo": "bar"}}

	tests := []struct {
		name       string
		in         *duckv1.WithPod
		configMaps []*corev1.ConfigMap
		sbStatus   *SinkBindingStatus
		want       *duckv1.WithPod
		ctx        context.Context
	}{{
		name: "nothing to add",
		in: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "K_SINK",
								Value: destination.URI.String(),
							}, {
								Name:  "K_CA_CERTS",
								Value: caCert,
							}, {
								Name:  "K_CE_OVERRIDES",
								Value: `{"extensions":{"foo":"bar"}}`,
							}},
						}},
					},
				},
			},
		},
		want: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "K_SINK",
								Value: destination.URI.String(),
							}, {
								Name:  "K_CA_CERTS",
								Value: caCert,
							}, {
								Name:  "K_CE_OVERRIDES",
								Value: `{"extensions":{"foo":"bar"}}`,
							}},
						}},
					},
				},
			},
		},
	}, {
		name: "add trust bundles",
		want: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "kne-bundle-knative-eventing-bundle" + strings.Repeat("a", 29),
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "knative-eventing-bundle" + strings.Repeat("a", 29),
										},
									},
								},
							},
						},
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "K_SINK",
								Value: destination.URI.String(),
							}, {
								Name:  "K_CA_CERTS",
								Value: caCert,
							}, {
								Name:  "K_CE_OVERRIDES",
								Value: `{"extensions":{"foo":"bar"}}`,
							}},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "kne-bundle-knative-eventing-bundle" + strings.Repeat("a", 29),
									MountPath: "/knative-custom-certs/knative-eventing-bundle" + strings.Repeat("a", 29),
									ReadOnly:  true,
								},
							},
						}},
					},
				},
			},
		},
		in: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "K_SINK",
								Value: destination.URI.String(),
							}, {
								Name:  "K_CA_CERTS",
								Value: caCert,
							}, {
								Name:  "K_CE_OVERRIDES",
								Value: `{"extensions":{"foo":"bar"}}`,
							}},
						}},
					},
				},
			},
		},
		configMaps: []*corev1.ConfigMap{
			{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "knative-eventing",
					Name:      "knative-eventing-bundle" + strings.Repeat("a", 29),
					Labels: map[string]string{
						"networking.knative.dev/trust-bundle": "true",
					},
				},
				Immutable: nil,
				Data: map[string]string{
					"knative-eventing-bundle.pem": "something",
				},
			},
		},
	}, {
		name: "add trust bundles - long CM name",
		want: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "kne-bundle-7840a1e43e73e2ce40d1180208cba2a6knative-eventing-bun",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "knative-eventing-bundle" + strings.Repeat("a", 30),
										},
									},
								},
							},
						},
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "K_SINK",
								Value: destination.URI.String(),
							}, {
								Name:  "K_CA_CERTS",
								Value: caCert,
							}, {
								Name:  "K_CE_OVERRIDES",
								Value: `{"extensions":{"foo":"bar"}}`,
							}},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "kne-bundle-7840a1e43e73e2ce40d1180208cba2a6knative-eventing-bun",
									MountPath: "/knative-custom-certs/knative-eventing-bundle" + strings.Repeat("a", 30),
									ReadOnly:  true,
								},
							},
						}},
					},
				},
			},
		},
		in: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "K_SINK",
								Value: destination.URI.String(),
							}, {
								Name:  "K_CA_CERTS",
								Value: caCert,
							}, {
								Name:  "K_CE_OVERRIDES",
								Value: `{"extensions":{"foo":"bar"}}`,
							}},
						}},
					},
				},
			},
		},
		configMaps: []*corev1.ConfigMap{
			{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "knative-eventing",
					Name:      "knative-eventing-bundle" + strings.Repeat("a", 30),
					Labels: map[string]string{
						"networking.knative.dev/trust-bundle": "true",
					},
				},
				Immutable: nil,
				Data: map[string]string{
					"knative-eventing-bundle.pem": "something",
				},
			},
		},
	}, {
		name: "fix the URI",
		in: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "K_SINK",
								Value: "the wrong value",
							}, {
								Name:  "K_CA_CERTS",
								Value: "wrong value",
							}, {
								Name:  "K_CE_OVERRIDES",
								Value: `{"extensions":{"wrong":"value"}}`,
							}},
						}},
					},
				},
			},
		},
		want: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "K_SINK",
								Value: destination.URI.String(),
							}, {
								Name:  "K_CA_CERTS",
								Value: caCert,
							}, {
								Name:  "K_CE_OVERRIDES",
								Value: `{"extensions":{"foo":"bar"}}`,
							}},
						}},
					},
				},
			},
		},
	}, {
		name: "lots to add",
		in: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{{
							Name:  "setup",
							Image: "busybox",
						}},
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "FOO",
								Value: "BAR",
							}, {
								Name:  "BAZ",
								Value: "INGA",
							}},
						}, {
							Name:  "sidecar",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "BAZ",
								Value: "INGA",
							}},
						}},
					},
				},
			},
		},
		want: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{{
							Name:  "setup",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "K_SINK",
								Value: destination.URI.String(),
							}, {
								Name:  "K_CA_CERTS",
								Value: caCert,
							}, {
								Name:  "K_CE_OVERRIDES",
								Value: `{"extensions":{"foo":"bar"}}`,
							}},
						}},
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "FOO",
								Value: "BAR",
							}, {
								Name:  "BAZ",
								Value: "INGA",
							}, {
								Name:  "K_SINK",
								Value: destination.URI.String(),
							}, {
								Name:  "K_CA_CERTS",
								Value: caCert,
							}, {
								Name:  "K_CE_OVERRIDES",
								Value: `{"extensions":{"foo":"bar"}}`,
							}},
						}, {
							Name:  "sidecar",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "BAZ",
								Value: "INGA",
							}, {
								Name:  "K_SINK",
								Value: destination.URI.String(),
							}, {
								Name:  "K_CA_CERTS",
								Value: caCert,
							}, {
								Name:  "K_CE_OVERRIDES",
								Value: `{"extensions":{"foo":"bar"}}`,
							}},
						}},
					},
				},
			},
		},
	}, {
		name: "adds OIDC token volume",
		in: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{{
							Name:  "init",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "K_SINK",
								Value: destination.URI.String(),
							}, {
								Name:  "K_CA_CERTS",
								Value: caCert,
							}, {
								Name:  "K_CE_OVERRIDES",
								Value: `{"extensions":{"foo":"bar"}}`,
							}},
						}},
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "K_SINK",
								Value: destination.URI.String(),
							}, {
								Name:  "K_CA_CERTS",
								Value: caCert,
							}, {
								Name:  "K_CE_OVERRIDES",
								Value: `{"extensions":{"foo":"bar"}}`,
							}},
						}},
					},
				},
			},
		},
		sbStatus: &SinkBindingStatus{
			OIDCTokenSecretName: pointer.String("oidc-token"),
		},
		want: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{{
							Name:  "init",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "K_SINK",
								Value: destination.URI.String(),
							}, {
								Name:  "K_CA_CERTS",
								Value: caCert,
							}, {
								Name:  "K_CE_OVERRIDES",
								Value: `{"extensions":{"foo":"bar"}}`,
							}},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      oidcTokenVolumeName,
								MountPath: "/oidc",
							}},
						}},
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "K_SINK",
								Value: destination.URI.String(),
							}, {
								Name:  "K_CA_CERTS",
								Value: caCert,
							}, {
								Name:  "K_CE_OVERRIDES",
								Value: `{"extensions":{"foo":"bar"}}`,
							}},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      oidcTokenVolumeName,
								MountPath: "/oidc",
							}},
						}},
						Volumes: []corev1.Volume{{
							Name: oidcTokenVolumeName,
							VolumeSource: corev1.VolumeSource{
								Projected: &corev1.ProjectedVolumeSource{
									Sources: []corev1.VolumeProjection{{
										Secret: &corev1.SecretProjection{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "oidc-token",
											},
										},
									}},
								},
							},
						}},
					},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.in
			applicationContext, _ := fakedynamicclient.With(context.Background(), scheme.Scheme, got)
			applicationContext = addressable.WithDuck(applicationContext)
			r := resolver.NewURIResolverFromTracker(applicationContext, tracker.New(func(types.NamespacedName) {}, 0))

			ctx, _ := SetupFakeContext(t)
			if test.ctx != nil {
				ctx = test.ctx
			}
			ctx = WithURIResolver(ctx, r)
			ctx = WithTrustBundleConfigMapLister(ctx, configmapinformer.Get(ctx).Lister())

			for _, cm := range test.configMaps {
				_ = configmapinformer.Get(ctx).Informer().GetIndexer().Add(cm)
			}

			sb := &SinkBinding{
				Spec: SinkBindingSpec{
					SourceSpec: duckv1.SourceSpec{
						Sink:                destination,
						CloudEventOverrides: &overrides,
					}},
			}

			if test.sbStatus != nil {
				sb.Status = *test.sbStatus
			}

			sb.Do(ctx, got)

			if !cmp.Equal(got, test.want) {
				t.Error("Undo (-want, +got):", cmp.Diff(test.want, got))
			}
		})
	}
}

func TestSinkBindingDoNoURI(t *testing.T) {
	want := &duckv1.WithPod{
		Spec: duckv1.WithPodSpec{
			Template: duckv1.PodSpecable{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "blah",
						Image: "busybox",
						Env:   []corev1.EnvVar{},
					}},
				},
			},
		},
	}
	got := &duckv1.WithPod{
		Spec: duckv1.WithPodSpec{
			Template: duckv1.PodSpecable{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "blah",
						Image: "busybox",
						Env: []corev1.EnvVar{{
							Name:  "K_SINK",
							Value: "this should be removed",
						}, {
							Name:  "K_CA_CERTS",
							Value: "this should be removed",
						}, {
							Name:  "K_CE_OVERRIDES",
							Value: `{"extensions":{"tobe":"removed"}}`,
						}},
					}},
				},
			},
		},
	}

	sb := &SinkBinding{}
	sb.Do(context.Background(), got)

	if !cmp.Equal(got, want) {
		t.Error("Undo (-want, +got):", cmp.Diff(want, got))
	}
}
