/*
Copyright 2024 The Knative Authors

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

package crossnamespace

import (
	"context"
	"fmt"
	"strings"

	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
)

type ResourceInfo interface {
	duckv1.KRShaped
	GetCrossNamespaceRef() duckv1.KReference
}

func CheckNamespace(ctx context.Context, r ResourceInfo) *apis.FieldError {
	targetKind := r.GetCrossNamespaceRef().Kind
	targetGroup := r.GetCrossNamespaceRef().Group
	targetName := r.GetCrossNamespaceRef().Name
	targetNamespace := r.GetCrossNamespaceRef().Namespace
	targetFieldName := fmt.Sprintf("spec.%sNamespace", targetKind)

	if targetGroup == "" {
		targetGroup = strings.Split(r.GetCrossNamespaceRef().APIVersion, "/")[0]
	}

	// If the target namespace is empty or the same as the object namespace, this function is skipped
	if targetNamespace == "" || targetNamespace == r.GetNamespace() {
		return nil
	}

	// GetUserInfo accesses the UserInfo attached to the webhook context.
	userInfo := apis.GetUserInfo(ctx)
	if userInfo == nil {
		return &apis.FieldError{
			Paths:   []string{targetFieldName},
			Message: "failed to get userInfo, which is needed to validate access to the target namespace",
		}
	}

	client := kubeclient.Get(ctx)

	// SubjectAccessReview checks if the user is authorized to perform an action.
	action := authv1.ResourceAttributes{
		Name:      targetName,
		Namespace: targetNamespace,
		Verb:      "knsubscribe",
		Group:     targetGroup,
		Resource:  targetKind,
	}

	// Create the SubjectAccessReview
	check := authv1.SubjectAccessReview{
		Spec: authv1.SubjectAccessReviewSpec{
			ResourceAttributes: &action,
			User:               userInfo.Username,
			Groups:             userInfo.Groups,
		},
	}

	resp, err := client.AuthorizationV1().SubjectAccessReviews().Create(ctx, &check, metav1.CreateOptions{})

	if err != nil {
		return &apis.FieldError{
			Paths:   []string{targetFieldName},
			Message: fmt.Sprintf("failed to make authorization request to see if user can subscribe to resources in namespace: %s", err.Error()),
		}
	}

	if !resp.Status.Allowed {
		return &apis.FieldError{
			Paths:   []string{targetFieldName},
			Message: fmt.Sprintf("user %s is not authorized to get target resource in namespace: %s", userInfo.Username, targetNamespace),
		}
	}

	return nil
}
