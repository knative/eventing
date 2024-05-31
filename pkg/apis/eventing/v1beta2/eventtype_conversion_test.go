/*
Copyright 2021 The Knative Authors

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

package v1beta2

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/eventing/pkg/apis/eventing/v1beta3"
)

func TestEventTypeConversionHighestVersion(t *testing.T) {
	good, bad := &EventType{}, &EventType{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

func TestEventTypeConversionV1Beta3(t *testing.T) {
	in := &EventType{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-name",
			Namespace: "my-ns",
			UID:       "1234",
		},
		Spec: EventTypeSpec{
			Type:   "t1",
			Source: &apis.URL{Scheme: "https", Host: "127.0.0.1", Path: "/sources/my-source"},
			Schema: &apis.URL{Scheme: "https", Host: "127.0.0.1", Path: "/schemas/my-schema"},
			Broker: "",
			Reference: &duckv1.KReference{
				Kind:       "Broker",
				Name:       "my-broker",
				APIVersion: "eventing.knative.dev/v1",
			},
			Description: "my-description",
		},
		Status: EventTypeStatus{
			Status: duckv1.Status{
				ObservedGeneration: 1234,
			},
		},
	}

	expected := &v1beta3.EventType{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-name",
			Namespace: "my-ns",
			UID:       "1234",
		},
		Spec: v1beta3.EventTypeSpec{
			Reference:   in.Spec.Reference.DeepCopy(),
			Description: in.Spec.Description,
			Attributes: []v1beta3.EventAttributeDefinition{
				{
					Name:     "type",
					Required: true,
					Value:    in.Spec.Type,
				},
				{
					Name:     "schemadata",
					Required: false,
					Value:    in.Spec.Schema.String(),
				},
				{
					Name:     "source",
					Required: true,
					Value:    in.Spec.Source.String(),
				},
			},
		},
		Status: v1beta3.EventTypeStatus{
			Status: duckv1.Status{
				ObservedGeneration: 1234,
			},
		},
	}
	got := &v1beta3.EventType{}

	if err := in.ConvertTo(context.Background(), got); err != nil {
		t.Errorf("ConvertTo() = %#v, wanted no error, got %#v", expected, err)
	} else if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("ConvertTo(), (-want, +got)\n%s", diff)
	}

	from := &EventType{}
	if err := from.ConvertFrom(context.Background(), expected); err != nil {
		t.Errorf("ConvertFrom() = %#v, wanted no error %#v", in, err)
	} else if diff := cmp.Diff(in, from); diff != "" {
		t.Errorf("ConvertFrom(), (-want, +got)\n%s", diff)
	}
}
