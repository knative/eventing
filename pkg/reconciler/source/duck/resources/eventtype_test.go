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

package resources

import (
	"crypto/md5"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1beta2"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"
)

func TestMakeEventType(t *testing.T) {
	args := &EventTypeArgs{
		Source: &duckv1.Source{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "source-name",
				Namespace: "source-namespace",
				UID:       "source-uid",
			},
			TypeMeta: metav1.TypeMeta{
				APIVersion: "testing.sources.knative.dev/v1beta2",
				Kind:       "TestSource",
			},
			Spec: duckv1.SourceSpec{
				Sink: duckv1.Destination{
					Ref: &duckv1.KReference{
						Name: "sink-name",
					},
				},
			},
		},
		CeType:      "my-type",
		CeSource:    apis.HTTP("my-source"),
		CeSchema:    apis.HTTP("my-schema"),
		Description: "my-description",
	}

	got := MakeEventType(args)

	want := &v1beta2.EventType{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%x", md5.Sum([]byte("my-type"+"http://my-source"+"http://my-schema"+"source-uid"))),
			Labels:    Labels("source-name"),
			Namespace: "source-namespace",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "testing.sources.knative.dev/v1beta2",
				Kind:               "TestSource",
				Name:               "source-name",
				UID:                "source-uid",
				BlockOwnerDeletion: ptr.Bool(true),
				Controller:         ptr.Bool(true),
			}},
		},
		Spec: v1beta2.EventTypeSpec{
			Type:        "my-type",
			Source:      apis.HTTP("my-source"),
			Schema:      apis.HTTP("my-schema"),
			Description: "my-description",
			Broker:      "sink-name",
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Error("unexpected difference (-want, +got) =", diff)
	}
}
