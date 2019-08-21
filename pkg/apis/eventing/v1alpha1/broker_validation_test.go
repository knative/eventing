/*
Copyright 2019 The Knative Authors

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
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/apis"
)

// No-op test because method does nothing.
func TestBrokerValidation(t *testing.T) {
	b := Broker{}
	_ = b.Validate(context.TODO())
}

// No-op test because method does nothing.
func TestBrokerSpecValidation(t *testing.T) {
	bs := BrokerSpec{}
	_ = bs.Validate(context.TODO())
}

// No-op test because method does nothing.
func TestBrokerImmutableFields(t *testing.T) {
	original := &Broker{}
	current := &Broker{}
	_ = current.CheckImmutableFields(context.TODO(), original)
}

func TestValidSpec(t *testing.T) {
	tests := []struct {
		name string
		spec BrokerSpec
		want *apis.FieldError
	}{{
		name: "valid empty",
		spec: BrokerSpec{},
		want: nil,
	}, {
		name: "valid provider",
		spec: BrokerSpec{
			ChannelTemplate: &eventingduckv1alpha1.ChannelTemplateSpec{
				TypeMeta: metav1.TypeMeta{APIVersion: "myapiversion", Kind: "mykind"},
			},
		},
		want: nil,
	}, {
		name: "invalid templatespec, missing kind",
		spec: BrokerSpec{
			ChannelTemplate: &eventingduckv1alpha1.ChannelTemplateSpec{TypeMeta: metav1.TypeMeta{APIVersion: "myapiversion"}},
		},
		want: func() *apis.FieldError {
			var errs *apis.FieldError
			fe := apis.ErrMissingField("channelTemplateSpec.kind")
			errs = errs.Also(fe)
			return errs
		}(),
	}, {
		name: "invalid templatespec, missing apiVersion",
		spec: BrokerSpec{
			ChannelTemplate: &eventingduckv1alpha1.ChannelTemplateSpec{TypeMeta: metav1.TypeMeta{Kind: "mykind"}},
		},
		want: func() *apis.FieldError {
			var errs *apis.FieldError
			fe := apis.ErrMissingField("channelTemplateSpec.apiVersion")
			errs = errs.Also(fe)
			return errs
		}(),
	}, {
		name: "valid templatespec",
		spec: BrokerSpec{
			ChannelTemplate: &eventingduckv1alpha1.ChannelTemplateSpec{TypeMeta: metav1.TypeMeta{Kind: "mykind", APIVersion: "myapiversion"}},
		},
		want: nil,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.spec.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("BrokerSpec.Validate (-want, +got) = %v", diff)
			}
		})
	}
}
