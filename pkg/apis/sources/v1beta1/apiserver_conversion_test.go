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
	"errors"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

type dummyObject struct{}

func (*dummyObject) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	return errors.New("Won't go")
}

func (*dummyObject) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	return errors.New("Won't go")
}

func TestApiServerSourceConversionBadType(t *testing.T) {
	good, bad := &ApiServerSource{}, &dummyObject{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

func TestApiServerSourceConversionRoundTripUp(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&v1.ApiServerSource{}}

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
				SourceSpec: duckv1.SourceSpec{
					Sink: sink,
				},
				Resources: []APIVersionKindSelector{{
					APIVersion: "A1",
					Kind:       "K1",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"A1": "K1"},
					},
				}, {
					APIVersion: "A2",
					Kind:       "K2",
				}},
				EventMode: "Ref",
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
				SourceSpec: duckv1.SourceSpec{
					Sink: sink,
					CloudEventOverrides: &duckv1.CloudEventOverrides{
						Extensions: map[string]string{
							"foo": "bar",
							"baz": "baf",
						},
					},
				},
				Resources: []APIVersionKindSelector{{
					APIVersion: "A1",
					Kind:       "K1",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
						},
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "aKey",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"the", "house"},
						}},
					},
				}, {
					APIVersion: "A2",
					Kind:       "K2",
				}},
				ResourceOwner: &APIVersionKind{
					APIVersion: "custom/v1",
					Kind:       "Parent",
				},
				EventMode:          "Resource",
				ServiceAccountName: "adult",
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
				if diff := cmp.Diff(test.in, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}

// This tests round tripping from a higher version -> v1beta1 and back to the higher version.
func TestApiServerSourceConversionRoundTripDown(t *testing.T) {
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
	}{{name: "empty",
		in: &v1.ApiServerSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "apiserver-name",
				Namespace:  "apiserver-ns",
				Generation: 17,
			},
			Spec:   v1.ApiServerSourceSpec{},
			Status: v1.ApiServerSourceStatus{},
		},
	}, {name: "simple configuration",
		in: &v1.ApiServerSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "apiserver-name",
				Namespace:  "apiserver-ns",
				Generation: 17,
			},
			Spec: v1.ApiServerSourceSpec{
				SourceSpec: duckv1.SourceSpec{
					Sink: sink,
				},
				Resources: []v1.APIVersionKindSelector{{
					APIVersion: "A1",
					Kind:       "K1",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"A1": "K1"},
					},
				}, {
					APIVersion: "A2",
					Kind:       "K2",
				}},
				EventMode: "Ref",
			},
			Status: v1.ApiServerSourceStatus{
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
		in: &v1.ApiServerSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "apiserver-name",
				Namespace:  "apiserver-ns",
				Generation: 17,
			},
			Spec: v1.ApiServerSourceSpec{
				SourceSpec: duckv1.SourceSpec{
					Sink:                sink,
					CloudEventOverrides: &ceOverrides,
				},
				Resources: []v1.APIVersionKindSelector{{
					APIVersion: "A1",
					Kind:       "K1",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
						},
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "aKey",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"the", "house"},
						}},
					},
				}, {
					APIVersion: "A2",
					Kind:       "K2",
				}},
				ResourceOwner: &v1.APIVersionKind{
					APIVersion: "custom/v1",
					Kind:       "Parent",
				},
				EventMode:          "Resource",
				ServiceAccountName: "adult",
			},
			Status: v1.ApiServerSourceStatus{
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
