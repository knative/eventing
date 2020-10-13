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

package helpers

import (
	"context"
	"fmt"

	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	testlib "knative.dev/eventing/test/lib"
)

func ServiceAccountCanDoVerbOnResourceOrFail(client *testlib.Client, gvr schema.GroupVersionResource, subresource string, saName string, verb string) {
	r, err := client.Kube.AuthorizationV1().SubjectAccessReviews().Create(context.Background(), &authv1.SubjectAccessReview{
		Spec: authv1.SubjectAccessReviewSpec{
			User: fmt.Sprintf("system:serviceaccount:%s:%s", client.Namespace, saName),
			ResourceAttributes: &authv1.ResourceAttributes{
				Verb:        verb,
				Group:       gvr.Group,
				Version:     gvr.Version,
				Resource:    gvr.Resource,
				Subresource: subresource,
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		client.T.Fatalf("Error while checking if %q is not allowed on %s.%s/%s subresource:%q. err: %q", verb, gvr.Resource, gvr.Group, gvr.Version, subresource, err)
	}
	if !r.Status.Allowed {
		client.T.Fatalf("Operation %q is not allowed on %s.%s/%s subresource:%q", verb, gvr.Resource, gvr.Group, gvr.Version, subresource)
	}
}
