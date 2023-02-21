/*
Copyright 2023 The Knative Authors

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

package workloads

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
)

type Workload struct {
	// GVR is the group, version, resource of the workload.
	GVR schema.GroupVersionResource
	// Name is the full name of the workload.
	Name string
	// PartialName is the partial name of the workload it is useful when the full name is not known.
	// It is only used when Name is not specified.
	PartialName string
	// Namespace is the namespace where the workload is located.
	// Empty Namespace will default to use the test namespace pulled from the environment.
	Namespace string
	// Selector is the workload selector
	Selector metav1.LabelSelector
}

func (w Workload) String() string {
	name := w.Name
	if w.Name == "" {
		name = fmt.Sprintf("*%s*", w.PartialName)
	}
	return fmt.Sprintf("%s %s/%s", w.GVR.String(), w.Namespace, name)
}

func Selector(w Workload) *feature.Feature {
	f := feature.NewFeatureNamed("Workload " + w.String() + " match labels")
	f.Assert("Verify labels", SelectorStep(w))
	return f
}

func SelectorStep(w Workload) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		selector, err := metav1.LabelSelectorAsSelector(&w.Selector)
		if err != nil {
			t.Fatal(err)
		}

		if w.Namespace == "" {
			w.Namespace = environment.FromContext(ctx).Namespace()
		}

		resources, err := dynamicclient.Get(ctx).
			Resource(w.GVR).
			Namespace(w.Namespace).
			List(ctx, metav1.ListOptions{
				LabelSelector: selector.String(),
			})
		if err != nil {
			t.Errorf("Failed to get resource: %w", err)
			return
		}

		for _, r := range resources.Items {
			if r.GetName() == w.Name {
				return
			}
			if w.PartialName != "" && strings.Contains(r.GetName(), w.PartialName) {
				return
			}
		}

		t.Errorf("Selector %s doesn't match resource %s, resources:\n%s", w.Selector.String(), w.String(), resourcesSummary(resources.Items))
	}
}

type resourceSummary struct {
	GroupVersionKind schema.GroupVersionKind
	Namespace        string `json:"namespace"`
	Name             string `json:"name"`
	Labels           map[string]string
}

func resourcesSummary(items []unstructured.Unstructured) string {
	rs := make([]resourceSummary, 0, len(items))
	for _, i := range items {
		rs = append(rs, resourceSummary{
			GroupVersionKind: i.GroupVersionKind(),
			Namespace:        i.GetNamespace(),
			Name:             i.GetName(),
			Labels:           i.GetLabels(),
		})
	}
	bytes, _ := json.MarshalIndent(rs, "", "  ")
	return string(bytes)
}
