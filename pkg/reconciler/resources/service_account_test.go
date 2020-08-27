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
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"

	"knative.dev/eventing/pkg/apis/sources/v1beta1"
)

func TestNewServiceAccount(t *testing.T) {
	testNS := "test-ns"
	testName := "test-name"
	obj := &v1beta1.PingSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNS,
		},
	}

	want := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      testName,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(obj),
			},
		},
	}

	got := MakeServiceAccount(obj, testName)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected condition (-want, +got) = %v", diff)
	}
}
