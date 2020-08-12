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

package v1alpha2

import (
	"context"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/sources/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestContainerSourceConversionBadType(t *testing.T) {
	good, bad := &ContainerSource{}, &dummy{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

func TestContainerSourceConversionRoundTripUp(t *testing.T) {
	versions := []apis.Convertible{&v1beta1.ContainerSource{}}

	path := apis.HTTP("")
	path.Path = "/path"
	sink := duckv1.Destination{
		Ref: &duckv1.KReference{
			Kind:       "Foo",
			Namespace:  "Bar",
			Name:       "Baz",
			APIVersion: "Baf",
		},
		URI: path,
	}
	sinkUri := apis.HTTP("example.com")
	sinkUri.Path = "path"

	tests := []struct {
		name string
		in   *ContainerSource
	}{{name: "empty",
		in: &ContainerSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ping-name",
				Namespace:  "ping-ns",
				Generation: 17,
			},
			Spec: ContainerSourceSpec{},
			Status: ContainerSourceStatus{
				SourceStatus: duckv1.SourceStatus{
					Status: duckv1.Status{
						ObservedGeneration: 1,
						Conditions: duckv1.Conditions{{
							Type:   "Ready",
							Status: "True",
						}},
					},
				},
			},
		},
	}, {name: "simple configuration",
		in: &ContainerSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ping-name",
				Namespace:  "ping-ns",
				Generation: 17,
			},
			Spec: ContainerSourceSpec{
				SourceSpec: duckv1.SourceSpec{
					Sink: sink,
				},
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test",
							Image: "test-image",
						}},
					},
				},
			},
			Status: ContainerSourceStatus{
				SourceStatus: duckv1.SourceStatus{
					Status: duckv1.Status{
						ObservedGeneration: 1,
						Conditions: duckv1.Conditions{{
							Type:   "Ready",
							Status: "Unknown",
						}},
					},
					SinkURI: sinkUri,
				},
			},
		},
	}, {name: "full",
		in: &ContainerSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ping-name",
				Namespace:  "ping-ns",
				Generation: 17,
			},
			Spec: ContainerSourceSpec{
				SourceSpec: duckv1.SourceSpec{
					Sink: sink,
					CloudEventOverrides: &duckv1.CloudEventOverrides{
						Extensions: map[string]string{
							"foo": "bar",
							"baz": "baf",
						},
					},
				},
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test",
							Image: "test-image",
						}},
					},
				},
			},
			Status: ContainerSourceStatus{
				SourceStatus: duckv1.SourceStatus{
					Status: duckv1.Status{
						ObservedGeneration: 1,
						Conditions: duckv1.Conditions{{
							Type:   "Ready",
							Status: "True",
						}},
					},
					SinkURI: sinkUri,
				},
			},
		},
	}}
	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}

				got := &ContainerSource{}

				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}
				if diff := cmp.Diff(test.in, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}

// This tests round tripping from a higher version -> v1alpha2 and back to the higher version.
func TestContainerSourceConversionRoundTripDown(t *testing.T) {
	path := apis.HTTP("")
	path.Path = "/path"
	sink := duckv1.Destination{
		Ref: &duckv1.KReference{
			Kind:       "Foo",
			Namespace:  "Bar",
			Name:       "Baz",
			APIVersion: "Baf",
		},
		URI: path,
	}
	sinkUri := apis.HTTP("example.com")
	sinkUri.Path = "path"

	ceOverrides := duckv1.CloudEventOverrides{
		Extensions: map[string]string{
			"foo": "bar",
			"baz": "baf",
		},
	}

	tests := []struct {
		name string
		in   apis.Convertible
	}{{name: "full",
		in: &v1beta1.ContainerSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ping-name",
				Namespace:  "ping-ns",
				Generation: 17,
			},
			Spec: v1beta1.ContainerSourceSpec{
				SourceSpec: duckv1.SourceSpec{
					Sink:                sink,
					CloudEventOverrides: &ceOverrides,
				},
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test",
							Image: "test-image",
						}},
					},
				},
			},
			Status: v1beta1.ContainerSourceStatus{
				SourceStatus: duckv1.SourceStatus{
					Status: duckv1.Status{
						ObservedGeneration: 1,
						Conditions: duckv1.Conditions{{
							Type:   "Ready",
							Status: "True",
						}},
					},
					SinkURI: sinkUri,
				},
			},
		},
	}}
	for _, test := range tests {

		t.Run(test.name, func(t *testing.T) {
			down := &ContainerSource{}
			if err := down.ConvertFrom(context.Background(), test.in); err != nil {
				t.Errorf("ConvertTo() = %v", err)
			}

			got := (reflect.New(reflect.TypeOf(test.in).Elem()).Interface()).(apis.Convertible)

			if err := down.ConvertTo(context.Background(), got); err != nil {
				t.Errorf("ConvertFrom() = %v", err)
			}
			if diff := cmp.Diff(test.in, got); diff != "" {
				t.Errorf("roundtrip (-want, +got) = %v", diff)
			}
		})
	}
}
