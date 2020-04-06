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
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	"knative.dev/pkg/apis"
)

func TestBrokerImmutableFields(t *testing.T) {
	original := &Broker{
		Spec: BrokerSpec{
			ChannelTemplate: &messagingv1beta1.ChannelTemplateSpec{TypeMeta: metav1.TypeMeta{Kind: "my-kind"}},
		},
	}
	current := &Broker{
		Spec: BrokerSpec{
			ChannelTemplate: &messagingv1beta1.ChannelTemplateSpec{TypeMeta: metav1.TypeMeta{Kind: "my-other-kind"}},
		},
	}

	tests := map[string]struct {
		og      *Broker
		wantErr *apis.FieldError
	}{
		"nil original": {
			wantErr: nil,
		},
		"no ChannelTemplateSpec mutation": {
			og:      current,
			wantErr: nil,
		},
		"ChannelTemplateSpec mutated": {
			og: original,
			wantErr: &apis.FieldError{
				Message: "Immutable fields changed (-old +new)",
				Paths:   []string{"spec", "channelTemplate"},
				Details: `{*v1beta1.ChannelTemplateSpec}.TypeMeta.Kind:
	-: "my-kind"
	+: "my-other-kind"
`,
			},
		},
	}

	for n, test := range tests {
		t.Run(n, func(t *testing.T) {
			gotErr := current.CheckImmutableFields(context.Background(), test.og)
			if diff := cmp.Diff(test.wantErr.Error(), gotErr.Error()); diff != "" {
				t.Errorf("Broker.CheckImmutableFields (-want, +got) = %v", diff)
			}
		})
	}
}

func TestBrokerValidation(t *testing.T) {
	/* This test should fail: TODO: https://github.com/knative/eventing/issues/2128
	name: "invalid empty, missing channeltemplatespec",
	spec: BrokerSpec{},
	want: apis.ErrMissingField("channelTemplateSpec"),
	*/

	tests := []CRDTest{{
		name: "valid provider",
		cr: &Broker{
			Spec: BrokerSpec{
				ChannelTemplate: &messagingv1beta1.ChannelTemplateSpec{
					TypeMeta: metav1.TypeMeta{APIVersion: "myapiversion", Kind: "mykind"},
				},
			},
		},
		want: nil,
	}, {
		name: "invalid templatespec, missing kind",
		cr: &Broker{
			Spec: BrokerSpec{
				ChannelTemplate: &messagingv1beta1.ChannelTemplateSpec{TypeMeta: metav1.TypeMeta{APIVersion: "myapiversion"}},
			},
		},
		want: apis.ErrMissingField("kind").ViaField("spec.channelTemplateSpec"),
	}, {
		name: "invalid templatespec, missing apiVersion",
		cr: &Broker{
			Spec: BrokerSpec{
				ChannelTemplate: &messagingv1beta1.ChannelTemplateSpec{TypeMeta: metav1.TypeMeta{Kind: "mykind"}},
			},
		},
		want: apis.ErrMissingField("apiVersion").ViaField("spec.channelTemplateSpec"),
	}, {
		name: "valid templatespec",
		cr: &Broker{
			Spec: BrokerSpec{
				ChannelTemplate: &messagingv1beta1.ChannelTemplateSpec{TypeMeta: metav1.TypeMeta{Kind: "mykind", APIVersion: "myapiversion"}},
			},
		},
		want: nil,
	}}

	doValidateTest(t, tests)
}
