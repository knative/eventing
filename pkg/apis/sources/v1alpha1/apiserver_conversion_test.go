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

	corev1 "k8s.io/api/core/v1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"

	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
	"knative.dev/eventing/pkg/apis/sources/v1beta1"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestApiServerSourceConversionBadType(t *testing.T) {
	good, bad := &ApiServerSource{}, &dummy{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

func TestApiServerSourceConversionRoundTripUp(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&v1alpha2.ApiServerSource{}, &v1beta1.ApiServerSource{}}

	path, _ := apis.ParseURL("/path")
	sink := duckv1beta1.Destination{
		Ref: &corev1.ObjectReference{
			APIVersion: "Baf",
			Kind:       "Foo",
			Name:       "Baz",
			Namespace:  "Baz",
		},
		DeprecatedAPIVersion: "depApi",
		DeprecatedKind:       "depKind",
		DeprecatedName:       "depName",
		DeprecatedNamespace:  "depNamespace",
		URI:                  path,
	}
	sinkUri, _ := apis.ParseURL("http://example.com/path")

	tests := []struct {
		name string
		in   *ApiServerSource
	}{{name: "empty",
		in: &ApiServerSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "apiserver-name",
				Namespace:  "apiserver-ns",
				Generation: 17,
			},
			Spec: ApiServerSourceSpec{},
			Status: ApiServerSourceStatus{
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
		in: &ApiServerSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "apiserver-name",
				Namespace:  "apiserver-ns",
				Generation: 17,
			},
			Spec: ApiServerSourceSpec{
				Resources: []ApiServerResource{{
					APIVersion: "A1",
					Kind:       "K1",
					LabelSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"A1": "K1"},
					},
				}, {
					APIVersion: "A2",
					Kind:       "K2",
				}},
				Sink: &sink,
				Mode: "Ref",
			},
			Status: ApiServerSourceStatus{
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
		in: &ApiServerSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "apiserver-name",
				Namespace:  "apiserver-ns",
				Generation: 17,
			},
			Spec: ApiServerSourceSpec{
				Resources: []ApiServerResource{{
					APIVersion: "A1",
					Kind:       "K1",
					LabelSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
						},
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "aKey",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"the", "house"},
						}},
					},
					ControllerSelector: metav1.OwnerReference{},
					Controller:         true,
				}, {
					APIVersion: "A2",
					Kind:       "K2",
				}},
				ServiceAccountName: "adult",
				Sink:               &sink,
				CloudEventOverrides: &duckv1.CloudEventOverrides{
					Extensions: map[string]string{
						"foo": "bar",
						"baz": "baf",
					},
				},
				Mode: "Resource",
			},
			Status: ApiServerSourceStatus{
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

				got := &ApiServerSource{}

				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}
				fixed := fixApiServerSourceDeprecated(test.in)
				if diff := cmp.Diff(fixed, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}

// This tests round tripping from a higher version -> v1alpha1 and back to the higher version.
func TestApiServerSourceConversionRoundTripDown(t *testing.T) {
	path, _ := apis.ParseURL("/path")
	sink := duckv1.Destination{
		Ref: &duckv1.KReference{
			Kind:       "Foo",
			Namespace:  "Bar",
			Name:       "Baz",
			APIVersion: "Baf",
		},
		URI: path,
	}
	sinkURI, _ := apis.ParseURL("http://example.com/path")

	ceOverrides := duckv1.CloudEventOverrides{
		Extensions: map[string]string{
			"foo": "bar",
			"baz": "baf",
		},
	}

	tests := []struct {
		name string
		in   apis.Convertible
	}{{name: "empty",
		in: &v1beta1.ApiServerSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "apiserver-name",
				Namespace:  "apiserver-ns",
				Generation: 17,
			},
			Spec:   v1beta1.ApiServerSourceSpec{},
			Status: v1beta1.ApiServerSourceStatus{},
		},
	}, {name: "simple configuration",
		in: &v1beta1.ApiServerSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "apiserver-name",
				Namespace:  "apiserver-ns",
				Generation: 17,
			},
			Spec: v1beta1.ApiServerSourceSpec{
				SourceSpec: duckv1.SourceSpec{
					Sink: sink,
				},
			},
			Status: v1beta1.ApiServerSourceStatus{
				SourceStatus: duckv1.SourceStatus{
					Status: duckv1.Status{
						ObservedGeneration: 1,
						Conditions: duckv1.Conditions{{
							Type:   "Ready",
							Status: "True",
						}},
					},
					SinkURI: sinkURI,
				},
			},
		},
	}, {name: "full",
		in: &v1beta1.ApiServerSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "apiserver-name",
				Namespace:  "apiserver-ns",
				Generation: 17,
			},
			Spec: v1beta1.ApiServerSourceSpec{
				SourceSpec: duckv1.SourceSpec{
					Sink:                sink,
					CloudEventOverrides: &ceOverrides,
				},
			},
			Status: v1beta1.ApiServerSourceStatus{
				SourceStatus: duckv1.SourceStatus{
					Status: duckv1.Status{
						ObservedGeneration: 1,
						Conditions: duckv1.Conditions{{
							Type:   "Ready",
							Status: "True",
						}},
					},
					SinkURI: sinkURI,
				},
			},
		},
	}}
	for _, test := range tests {

		t.Run(test.name, func(t *testing.T) {
			down := &ApiServerSource{}
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

// Since v1alpha1 to v1alpha2 is lossy.
func fixApiServerSourceDeprecated(in *ApiServerSource) *ApiServerSource {
	for i := range in.Spec.Resources {
		in.Spec.Resources[i].Controller = false
		in.Spec.Resources[i].ControllerSelector = metav1.OwnerReference{}
	}
	if in.Spec.Sink != nil {
		in.Spec.Sink.DeprecatedAPIVersion = ""
		in.Spec.Sink.DeprecatedKind = ""
		in.Spec.Sink.DeprecatedName = ""
		in.Spec.Sink.DeprecatedNamespace = ""
	}
	return in
}
