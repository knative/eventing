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
	"errors"
	"reflect"
	"testing"

	"knative.dev/eventing/pkg/apis/sources/v1alpha2"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// TODO: Replace dummy some other Eventing object once they
// implement apis.Convertible
type dummy struct{}

func (*dummy) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	return errors.New("Won't go")
}

func (*dummy) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	return errors.New("Won't go")
}

func TestPingSourceConversionBadType(t *testing.T) {
	good, bad := &PingSource{}, &dummy{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

func TestPingSourceConversionRoundTripUp(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&v1alpha2.PingSource{}}

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
	sinkUri, _ := apis.ParseURL("http://example.com/path")

	tests := []struct {
		name string
		in   *PingSource
	}{{name: "empty",
		in: &PingSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ping-name",
				Namespace:  "ping-ns",
				Generation: 17,
			},
			Spec: PingSourceSpec{},
			Status: PingSourceStatus{
				Status: duckv1.Status{
					ObservedGeneration: 1,
					Conditions: duckv1.Conditions{{
						Type:   "Ready",
						Status: "True",
					}},
				},
			},
		},
	}, {name: "simple configuration",
		in: &PingSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ping-name",
				Namespace:  "ping-ns",
				Generation: 17,
			},
			Spec: PingSourceSpec{
				Sink: &sink,
			},
			Status: PingSourceStatus{
				Status: duckv1.Status{
					ObservedGeneration: 1,
					Conditions: duckv1.Conditions{{
						Type:   "Ready",
						Status: "Unknown",
					}},
				},
			},
		},
	}, {name: "full",
		in: &PingSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ping-name",
				Namespace:  "ping-ns",
				Generation: 17,
			},
			Spec: PingSourceSpec{
				// TODO: full spec
			},
			Status: PingSourceStatus{
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
	}}
	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}

				got := &PingSource{}

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

// This tests round tripping from a higher version -> v1alpha1 and back to the higher version.
func TestPingSourceConversionRoundTripDown(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.

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
		in: &v1alpha2.PingSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ping-name",
				Namespace:  "ping-ns",
				Generation: 17,
			},
			Spec:   v1alpha2.PingSourceSpec{},
			Status: v1alpha2.PingSourceStatus{},
		},
	}, {name: "simple configuration",
		in: &v1alpha2.PingSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ping-name",
				Namespace:  "ping-ns",
				Generation: 17,
			},
			Spec: v1alpha2.PingSourceSpec{
				SourceSpec: duckv1.SourceSpec{
					Sink: sink,
				},
			},
			Status: v1alpha2.PingSourceStatus{
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
		in: &v1alpha2.PingSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ping-name",
				Namespace:  "ping-ns",
				Generation: 17,
			},
			Spec: v1alpha2.PingSourceSpec{
				SourceSpec: duckv1.SourceSpec{
					Sink:                sink,
					CloudEventOverrides: &ceOverrides,
				},
				Schedule: "1 2 3 4 5",
				JsonData: `{"foo":"bar"}`,
			},
			Status: v1alpha2.PingSourceStatus{
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
			down := &PingSource{}
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
