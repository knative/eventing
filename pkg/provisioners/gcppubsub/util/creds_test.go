/*
Copyright 2018 The Knative Authors

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

package util

import (
	"context"
	"testing"

	"github.com/knative/eventing/pkg/provisioners/gcppubsub/util/testcreds"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetCredentials(t *testing.T) {
	testCases := map[string]struct {
		secretRef *v1.ObjectReference
		key       string
		secret    *v1.Secret
		err       bool
	}{
		"secretRef not found": {
			secretRef: &v1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  "some-other-namespace",
				Name:       "some-other-name",
			},
			secret: testcreds.MakeSecretWithCreds(),
			err:    true,
		},
		"secretRef key not present": {
			secretRef: testcreds.Secret,
			secret:    testcreds.MakeSecretWithCreds(),
			key:       "some-other-key",
			err:       true,
		},
		"unable to create credentials": {
			secretRef: testcreds.Secret,
			secret:    testcreds.MakeSecretWithInvalidCreds(),
			key:       testcreds.SecretKey,
			err:       true,
		},
		"success": {
			secretRef: testcreds.Secret,
			secret:    testcreds.MakeSecretWithCreds(),
			key:       testcreds.SecretKey,
			err:       false,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			client := fake.NewFakeClient(tc.secret)
			actual, actualErr := GetCredentials(context.TODO(), client, tc.secretRef, tc.key)
			if tc.err {
				if actualErr == nil {
					t.Fatalf("Expected an error.")
				}
				return
			}
			if actualErr != nil {
				t.Fatalf("Unexpected error: %v", actualErr)
			}
			if actual == nil {
				t.Fatal("creds should have been non-nil")
			}
		})
	}
}
