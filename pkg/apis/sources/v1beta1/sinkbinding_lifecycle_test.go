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

package v1beta1

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	fakedynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/tracker"
)

func init() {
	duckv1beta1.AddToScheme(scheme.Scheme)
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
		Version: "v1beta1",
		Kind:    "SinkBinding",
	}
	if got := r.GetGroupVersionKind(); got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestSinkBindingGetters(t *testing.T) {
	r := &SinkBinding{
		Spec: SinkBindingSpec{
			BindingSpec: duckv1beta1.BindingSpec{
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
			BindingSpec: duckv1beta1.BindingSpec{
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
	sink := apis.HTTP("table.ns.svc.cluster.local/flip")
	sink.Scheme = "uri"
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
			return s
		}(),
		want: true,
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
								Name:  "BAZ",
								Value: "INGA",
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
								Name:  "K_SINK",
								Value: "http://localhost:8080",
							}, {
								Name:  "BAZ",
								Value: "INGA",
							}, {
								Name:  "K_CE_OVERRIDES",
								Value: `{"extensions":{"foo":"bar"}}`,
							}},
						}, {
							Name:  "sidecar",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "K_SINK",
								Value: "http://localhost:8080",
							}, {
								Name:  "BAZ",
								Value: "INGA",
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
	}

	overrides := duckv1.CloudEventOverrides{Extensions: map[string]string{"foo": "bar"}}

	tests := []struct {
		name string
		in   *duckv1.WithPod
		want *duckv1.WithPod
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
								Name:  "K_CE_OVERRIDES",
								Value: `{"extensions":{"foo":"bar"}}`,
							}},
						}},
					},
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
								Name:  "K_CE_OVERRIDES",
								Value: `{"extensions":{"foo":"bar"}}`,
							}},
						}},
					},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.in
			ctx, _ := fakedynamicclient.With(context.Background(), scheme.Scheme, got)
			ctx = addressable.WithDuck(ctx)
			r := resolver.NewURIResolver(ctx, func(types.NamespacedName) {})
			ctx = WithURIResolver(context.Background(), r)

			sb := &SinkBinding{Spec: SinkBindingSpec{
				SourceSpec: duckv1.SourceSpec{
					Sink:                destination,
					CloudEventOverrides: &overrides,
				},
			}}
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
