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
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/sources/v1beta2"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestPingSourceConversionBadType(t *testing.T) {
	good, bad := &PingSource{}, &testObject{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Error("ConvertTo() = nil, wanted error")
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Error("ConvertFrom() = nil, wanted error")
	}
}

// This tests round tripping from v1beta1 -> a higher version and back to v1beta1.
func TestPingSourceConversionRoundTripUp(t *testing.T) {
	versions := []apis.Convertible{&v1beta2.PingSource{}}

	path := apis.HTTP("")
	path.Path = "/path"

	sinkUri := apis.HTTP("example.com")
	sinkUri.Path = "path"
	sink := duckv1.Destination{
		Ref: &duckv1.KReference{
			Kind:       "Foo",
			Namespace:  "Bar",
			Name:       "Baz",
			APIVersion: "Baf",
		},
		URI: path,
	}

	meta := metav1.ObjectMeta{
		Name:       "ping-name",
		Namespace:  "ping-ns",
		Generation: 17,
	}

	tests := []struct {
		name string
		in   *PingSource
	}{{
		"empty",
		&PingSource{
			ObjectMeta: meta,
			Spec:       PingSourceSpec{},
			Status:     PingSourceStatus{},
		},
	}, {
		"simple configuration",
		&PingSource{
			ObjectMeta: meta,
			Spec: PingSourceSpec{
				SourceSpec: duckv1.SourceSpec{
					Sink: sink,
				},
			},
			Status: PingSourceStatus{
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
	}, {
		"full with valid jsonData",
		&PingSource{
			ObjectMeta: meta,
			Spec: PingSourceSpec{
				SourceSpec: duckv1.SourceSpec{
					Sink: sink,
				},
				Schedule: "* * * * *",
				JsonData: `{"msg":"hey"}`,
			},
			Status: PingSourceStatus{
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
	}, {
		"full with jsonData that cannot be unmarshalled",
		&PingSource{
			ObjectMeta: meta,
			Spec: PingSourceSpec{
				SourceSpec: duckv1.SourceSpec{
					Sink: sink,
				},
				Schedule: "* * * * *",
				JsonData: "hello",
			},
			Status: PingSourceStatus{
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
	}, {
		"full with invalid v1beta2 spec annotation",
		&PingSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ping-name",
				Namespace:  "ping-ns",
				Generation: 17,
				Annotations: map[string]string{
					V1B2SpecAnnotationKey: "$$ invalid json $$",
				},
			},
			Spec: PingSourceSpec{
				SourceSpec: duckv1.SourceSpec{
					Sink: sink,
				},
				Schedule: "* * * * *",
				JsonData: "hello",
			},
			Status: PingSourceStatus{
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

				got := &PingSource{}

				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Error("ConvertFrom() =", err)
				}

				if diff := diffIgnoringAnnotations(test.in, got); diff != "" {
					t.Error("roundtrip (-want, +got) =", diff)
				}
			})
		}
	}
}

// This tests round tripping from a higher version -> v1beta1 and back to the higher version.
func TestPingSourceConversionRoundTripDown(t *testing.T) {
	path := apis.HTTP("")
	path.Path = "/path"

	sinkUri := apis.HTTP("example.com")
	sinkUri.Path = "path"
	sink := duckv1.Destination{
		Ref: &duckv1.KReference{
			Kind:       "Foo",
			Namespace:  "Bar",
			Name:       "Baz",
			APIVersion: "Baf",
		},
		URI: path,
	}

	ceOverrides := duckv1.CloudEventOverrides{
		Extensions: map[string]string{
			"foo": "bar",
			"baz": "baf",
		},
	}

	ceAttributes := []duckv1.CloudEventAttributes{{
		Type:   PingSourceEventType,
		Source: PingSourceSource("ping-ns", "ping-name"),
	}}

	tests := []struct {
		name string
		in   *v1beta2.PingSource
	}{{name: "empty",
		in: &v1beta2.PingSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ping-name",
				Namespace:  "ping-ns",
				Generation: 17,
			},
			Spec:   v1beta2.PingSourceSpec{},
			Status: v1beta2.PingSourceStatus{},
		},
	}, {name: "simple configuration",
		in: &v1beta2.PingSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ping-name",
				Namespace:  "ping-ns",
				Generation: 17,
			},
			Spec: v1beta2.PingSourceSpec{
				SourceSpec: duckv1.SourceSpec{
					Sink: sink,
				},
			},
			Status: v1beta2.PingSourceStatus{
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
	}, {name: "full: v1beta1 annotation contains valid json data",
		in: &v1beta2.PingSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ping-name",
				Namespace:  "ping-ns",
				Generation: 17,
			},
			Spec: v1beta2.PingSourceSpec{
				SourceSpec: duckv1.SourceSpec{
					Sink:                sink,
					CloudEventOverrides: &ceOverrides,
				},
				Schedule:    "1 2 3 4 5",
				Timezone:    "Knative/Land",
				ContentType: cloudevents.ApplicationJSON,
				Data:        `{"foo":"bar"}`,
			},
			Status: v1beta2.PingSourceStatus{
				SourceStatus: duckv1.SourceStatus{
					Status: duckv1.Status{
						ObservedGeneration: 1,
						Conditions: duckv1.Conditions{{
							Type:   "Ready",
							Status: "True",
						}},
					},
					SinkURI:              sinkUri,
					CloudEventAttributes: ceAttributes,
				},
			},
		},
	}, {name: "full: v1beta1 annotation contains invalid json data",
		in: &v1beta2.PingSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ping-name",
				Namespace:  "ping-ns",
				Generation: 17,
			},
			Spec: v1beta2.PingSourceSpec{
				SourceSpec: duckv1.SourceSpec{
					Sink:                sink,
					CloudEventOverrides: &ceOverrides,
				},
				Schedule:    "1 2 3 4 5",
				Timezone:    "Knative/Land",
				ContentType: cloudevents.ApplicationJSON,
				Data:        `{"body":"hello"}`,
			},
			Status: v1beta2.PingSourceStatus{
				SourceStatus: duckv1.SourceStatus{
					Status: duckv1.Status{
						ObservedGeneration: 1,
						Conditions: duckv1.Conditions{{
							Type:   "Ready",
							Status: "True",
						}},
					},
					SinkURI:              sinkUri,
					CloudEventAttributes: ceAttributes,
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			down := &PingSource{}
			if err := down.ConvertFrom(context.Background(), test.in); err != nil {
				t.Error("ConvertTo() =", err)
			}

			got := &v1beta2.PingSource{}

			if err := down.ConvertTo(context.Background(), got); err != nil {
				t.Error("ConvertFrom() =", err)
			}
			if diff := diffIgnoringAnnotations(test.in, got); diff != "" {
				t.Error("roundtrip (-want, +got) =", diff)
			}
		})
	}
}

// returns the diff of want and got, but ignoring the difference of annotations.
func diffIgnoringAnnotations(want metav1.Object, got metav1.Object) string {
	want.SetAnnotations(nil)
	got.SetAnnotations(nil)
	return cmp.Diff(want, got)
}

func TestPingSourceConversionFromHigherVersionNotDowngradable(t *testing.T) {
	tests := []struct {
		name string
		in   apis.Convertible
	}{{name: "v1beta1 spec annotation is invalid json",
		in: &v1beta2.PingSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "ping-name",
				Namespace:  "ping-ns",
				Generation: 17,
				Annotations: map[string]string{
					V1B1SpecAnnotationKey: "$$ invalid json $$",
				},
			},
			Spec:   v1beta2.PingSourceSpec{},
			Status: v1beta2.PingSourceStatus{},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out := &PingSource{}
			if err := out.ConvertFrom(context.Background(), test.in); err == nil {
				t.Errorf("ConvertFrom() = %#v, wanted error", out)
			}
		})
	}
}
