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

package sources

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/pkg/apis/duck"

	rbacv1 "k8s.io/api/rbac/v1"
)

var clusterRoleLabel = map[string]string{
	duck.SourceDuckVersionLabel: "true",
}

/*
	The test checks for the following in this order:
	1. Find cluster roles that match these criteria -
		a. Has label duck.knative.dev/source: "true",
		b. Has the eventing source in Resources for a Policy Rule, and
		c. Has all the expected verbs (get, list, watch)
*/
func SourceCRDRBACTestHelperWithComponentsTestRunner(
	t *testing.T,
	sourceTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption,
) {

	sourceTestRunner.RunTests(t, testlib.FeatureBasic, func(st *testing.T, source metav1.TypeMeta) {
		client := testlib.Setup(st, true, options...)
		defer testlib.TearDown(client)

		// From spec:
		// Each source MUST have the following:
		// kind: ClusterRole
		// apiVersion: rbac.authorization.k8s.io/v1
		// metadata:
		//   name: foos-source-observer
		//   labels:
		//     duck.knative.dev/source: "true"
		// rules:
		//   - apiGroups:
		//       - example.com
		//     resources:
		//       - foos
		//     verbs:
		//       - get
		//       - list
		//       - watch
		st.Run("Source CRD has source observer cluster role", func(t *testing.T) {
			ValidateRBAC(st, client, source)
		})

	})
}

func ValidateRBAC(st *testing.T, client *testlib.Client, object metav1.TypeMeta) {
	labelSelector := &metav1.LabelSelector{
		MatchLabels: clusterRoleLabel,
	}

	sourcePluralName := getSourcePluralName(client, object)

	//Spec: New sources MUST include a ClusterRole as part of installing themselves into a cluster.
	if !clusterRoleMeetsSpecs(client, labelSelector, sourcePluralName) {
		client.T.Fatalf("can't find source observer cluster role for CRD %q", object)
	}
}

func getSourcePluralName(client *testlib.Client, object metav1.TypeMeta) string {
	gvr, _ := meta.UnsafeGuessKindToResource(object.GroupVersionKind())
	crdName := gvr.Resource + "." + gvr.Group

	crd, err := client.Apiextensions.CustomResourceDefinitions().Get(context.Background(), crdName, metav1.GetOptions{
		TypeMeta: metav1.TypeMeta{},
	})
	if err != nil {
		client.T.Errorf("error while getting %q:%v", object, err)
	}
	return crd.Spec.Names.Plural
}

func clusterRoleMeetsSpecs(client *testlib.Client, labelSelector *metav1.LabelSelector, crdSourceName string) bool {
	crs, err := client.Kube.RbacV1().ClusterRoles().List(context.Background(), metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(), //Cluster Role with duck.knative.dev/source: "true" label
	})
	if err != nil {
		client.T.Errorf("error while getting cluster roles %v", err)
	}

	for _, cr := range crs.Items {
		for _, pr := range cr.Rules {
			if contains(pr.Resources, crdSourceName) && //Cluster Role has the eventing source listed in Resources for a Policy Rule
				((contains(pr.Verbs, "get") && contains(pr.Verbs, "list") && contains(pr.Verbs, "watch")) ||
					contains(pr.Verbs, rbacv1.VerbAll)) { //Cluster Role has all the expected Verbs
				return true
			}
		}
	}
	return false
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}
