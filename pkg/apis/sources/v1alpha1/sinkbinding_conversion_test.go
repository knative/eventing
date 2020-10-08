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

package v1alpha1

import (
	"context"
	"reflect"
	"testing"

	"knative.dev/pkg/tracker"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
	"knative.dev/eventing/pkg/apis/sources/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

func TestSinkBindingConversionBadType(t *testing.T) {
	good, bad := &SinkBinding{}, &dummyObject{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

func TestSinkBindingConversionRoundTripUp(t *testing.T) {
	versions := []apis.Convertible{&v1.SinkBinding{}, &v1beta1.SinkBinding{}, &v1alpha2.SinkBinding{}}

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

	subject := tracker.Reference{
		APIVersion: "API",
		Kind:       "K",
		Namespace:  "NS",
		Name:       "N",
	}

	tests := []struct {
		name string
		in   *SinkBinding
	}{{name: "empty",
		in: &SinkBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ping-name",
				Namespace:  "ping-ns",
				Generation: 17,
			},
			Spec: SinkBindingSpec{},
			Status: SinkBindingStatus{
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
		in: &SinkBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ping-name",
				Namespace:  "ping-ns",
				Generation: 17,
			},
			Spec: SinkBindingSpec{
				SourceSpec: duckv1.SourceSpec{
					Sink: sink,
				},
				BindingSpec: duckv1alpha1.BindingSpec{
					Subject: subject,
				},
			},
			Status: SinkBindingStatus{
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
		in: &SinkBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ping-name",
				Namespace:  "ping-ns",
				Generation: 17,
			},
			Spec: SinkBindingSpec{
				SourceSpec: duckv1.SourceSpec{
					Sink: sink,
					CloudEventOverrides: &duckv1.CloudEventOverrides{
						Extensions: map[string]string{
							"foo": "bar",
							"baz": "baf",
						},
					},
				},
				BindingSpec: duckv1alpha1.BindingSpec{
					Subject: subject,
				},
			},
			Status: SinkBindingStatus{
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
					t.Error("ConvertTo() =", err)
				}

				got := &SinkBinding{}

				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Error("ConvertFrom() =", err)
				}
				if diff := cmp.Diff(test.in, got); diff != "" {
					t.Error("roundtrip (-want, +got) =", diff)
				}
			})
		}
	}
}

// This tests round tripping from a higher version -> v1alpha1 and back to the higher version.
func TestSinkBindingConversionRoundTripDown(t *testing.T) {
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

	subject := tracker.Reference{
		APIVersion: "API",
		Kind:       "K",
		Namespace:  "NS",
		Name:       "N",
	}

	tests := []struct {
		name string
		in   apis.Convertible
	}{{name: "empty",
		in: &v1alpha2.SinkBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ping-name",
				Namespace:  "ping-ns",
				Generation: 17,
			},
			Spec:   v1alpha2.SinkBindingSpec{},
			Status: v1alpha2.SinkBindingStatus{},
		},
	}, {name: "simple configuration",
		in: &v1alpha2.SinkBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ping-name",
				Namespace:  "ping-ns",
				Generation: 17,
			},
			Spec: v1alpha2.SinkBindingSpec{
				SourceSpec: duckv1.SourceSpec{
					Sink: sink,
				},
			},
			Status: v1alpha2.SinkBindingStatus{
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
	}, {name: "full",
		in: &v1beta1.SinkBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ping-name",
				Namespace:  "ping-ns",
				Generation: 17,
			},
			Spec: v1beta1.SinkBindingSpec{
				SourceSpec: duckv1.SourceSpec{
					Sink:                sink,
					CloudEventOverrides: &ceOverrides,
				},
				BindingSpec: duckv1beta1.BindingSpec{Subject: subject},
			},
			Status: v1beta1.SinkBindingStatus{
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
			down := &SinkBinding{}
			if err := down.ConvertFrom(context.Background(), test.in); err != nil {
				t.Error("ConvertTo() =", err)
			}

			got := (reflect.New(reflect.TypeOf(test.in).Elem()).Interface()).(apis.Convertible)

			if err := down.ConvertTo(context.Background(), got); err != nil {
				t.Error("ConvertFrom() =", err)
			}
			if diff := cmp.Diff(test.in, got); diff != "" {
				t.Error("roundtrip (-want, +got) =", diff)
			}
		})
	}
}
