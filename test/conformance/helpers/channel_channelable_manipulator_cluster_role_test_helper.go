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
	"testing"

	"fmt"

	authv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/storage/names"

	"knative.dev/eventing/test/lib"
)

const aggregationClusterRoleName = "channelable-manipulator"

var permissionTestCaseVerbs = []string{"get", "list", "watch", "update", "patch"}

func TestChannelChannelableManipulatorClusterRoleTestRunner(
	t *testing.T,
	channelTestRunner lib.ChannelTestRunner,
	options ...lib.SetupClientOption,
) {

	channelTestRunner.RunTests(t, lib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := lib.Setup(st, true, options...)
		defer lib.TearDown(client)

		gvr, _ := meta.UnsafeGuessKindToResource(channel.GroupVersionKind())

		saName := names.SimpleNameGenerator.GenerateName("conformance-test-channel-manipulator-")
		client.CreateServiceAccountOrFail(saName)
		client.CreateClusterRoleBindingOrFail(
			saName,
			aggregationClusterRoleName,
			saName+"-cluster-role-binding",
		)
		client.WaitForAllTestResourcesReadyOrFail()

		for _, verb := range permissionTestCaseVerbs {
			t.Run(fmt.Sprintf("ChannelableManipulatorClusterRole can do %s on %s", verb, gvr), func(t *testing.T) {
				serviceAccountCanDoVerbOnResource(st, client, gvr, "", saName, verb)
			})
			t.Run(fmt.Sprintf("ChannelableManipulatorClusterRole can do %s on status subresource of %s", verb, gvr), func(t *testing.T) {
				serviceAccountCanDoVerbOnResource(st, client, gvr, "status", saName, verb)
			})
		}
	})
}

func serviceAccountCanDoVerbOnResource(st *testing.T, client *lib.Client, gvr schema.GroupVersionResource, subresource string, saName string, verb string) {
	// From spec: (...) ClusterRole MUST include permissions to create, get, list, watch, patch,
	// and update the CRD's custom objects and their status.
	allowed, err := isAllowed(saName, client, verb, gvr, subresource)
	if err != nil {
		client.T.Fatalf("Error while checking if %q is not allowed on %s.%s/%s subresource:%q. err: %q", verb, gvr.Resource, gvr.Group, gvr.Version, subresource, err)
	}
	if !allowed {
		client.T.Fatalf("Operation %q is not allowed on %s.%s/%s subresource:%q", verb, gvr.Resource, gvr.Group, gvr.Version, subresource)
	}
}

func isAllowed(saName string, client *lib.Client, verb string, gvr schema.GroupVersionResource, subresource string) (bool, error) {

	r, err := client.Kube.Kube.AuthorizationV1().SubjectAccessReviews().Create(&authv1.SubjectAccessReview{
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
	})
	if err != nil {
		return false, err
	}
	return r.Status.Allowed, nil
}
