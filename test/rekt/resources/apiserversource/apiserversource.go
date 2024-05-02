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

package apiserversource

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime/schema"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"

	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/pkg/client/injection/kube/client"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/reconciler-test/pkg/manifest"

	yamllib "sigs.k8s.io/yaml"
)

//go:embed *.yaml
var yaml embed.FS

func Gvr() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "sources.knative.dev", Version: "v1", Resource: "apiserversources"}
}

// IsReady tests to see if an ApiServerSource becomes ready within the time given.
func IsReady(name string, timings ...time.Duration) feature.StepFn {
	return k8s.IsReady(Gvr(), name, timings...)
}

// Install returns a step function which creates an ApiServerSource resource, augmented with the config fn options.
func Install(name string, opts ...manifest.CfgFn) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		if _, err := InstallLocalYaml(ctx, name, opts...); err != nil {
			t.Error(err)
		}
	}
}

// InstallLocalYaml will create a ApiServerSource resource, augmented with the config fn options.
func InstallLocalYaml(ctx context.Context, name string, opts ...manifest.CfgFn) (manifest.Manifest, error) {
	cfg := map[string]interface{}{
		"name": name,
	}
	for _, fn := range opts {
		fn(cfg)
	}
	return manifest.InstallYamlFS(ctx, yaml, cfg)
}

// WithServiceAccountName sets the service account name on the ApiServerSource spec.
func WithServiceAccountName(serviceAccountName string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["serviceAccountName"] = serviceAccountName
	}
}

// WithEventMode sets the event mode on the ApiServerSource spec.
func WithEventMode(eventMode string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["mode"] = eventMode
	}
}

// WithSink adds the sink related config to a ApiServerSource spec.
func WithSink(d *duckv1.Destination) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["sink"]; !set {
			cfg["sink"] = map[string]interface{}{}
		}
		sink := cfg["sink"].(map[string]interface{})

		ref := d.Ref
		uri := d.URI

		if d.CACerts != nil {
			// This is a multi-line string and should be indented accordingly.
			// Replace "new line" with "new line + spaces".
			sink["CACerts"] = strings.ReplaceAll(*d.CACerts, "\n", "\n      ")
		}

		if uri != nil {
			sink["uri"] = uri.String()
		}

		if d.Audience != nil {
			sink["audience"] = *d.Audience
		}

		if ref != nil {
			if _, set := sink["ref"]; !set {
				sink["ref"] = map[string]interface{}{}
			}
			sref := sink["ref"].(map[string]interface{})
			sref["apiVersion"] = ref.APIVersion
			sref["kind"] = ref.Kind
			sref["namespace"] = ref.Namespace
			sref["name"] = ref.Name
		}
	}
}

// WithResources adds the resources related config to a ApiServerSource spec.
func WithResources(resources ...v1.APIVersionKindSelector) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if _, set := cfg["resources"]; !set {
			cfg["resources"] = []map[string]interface{}{}
		}

		for _, resource := range resources {
			elem := map[string]interface{}{
				"apiVersion": resource.APIVersion,
				"kind":       resource.Kind,
				"selector":   labelSelectorToStringMap(resource.LabelSelector),
			}
			cfg["resources"] = append(cfg["resources"].([]map[string]interface{}), elem)
		}
	}
}

// WithNamespaceSelector adds a namespace selector to an ApiServerSource spec.
func WithNamespaceSelector(selector *metav1.LabelSelector) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["namespaceSelector"] = labelSelectorToStringMap(selector)
	}
}

func labelSelectorToStringMap(selector *metav1.LabelSelector) map[string]interface{} {
	if selector == nil {
		return nil
	}

	r := map[string]interface{}{}

	r["matchLabels"] = selector.MatchLabels

	if selector.MatchExpressions != nil {
		me := []map[string]interface{}{}
		for _, ml := range selector.MatchExpressions {
			me = append(me, map[string]interface{}{
				"key":      ml.Key,
				"operator": ml.Operator,
				"values":   ml.Values,
			})
		}
		r["matchExpressions"] = me
	}

	return r
}

func MatchLabels(labels1 map[string]string, labels2 map[string]string) bool {
	for k, v := range labels1 {
		if labels2[k] != v {
			return false
		}
	}
	return true
}

func TestLabels() map[string]string {
	return map[string]string{
		"testkey":  "testvalue",
		"testkey1": "testvalue1",
		"testkey2": "testvalue2",
	}
}

func VerifyNodeSelectorDeployment(source string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		env := environment.FromContext(ctx)
		ns := env.Namespace()

		kubeClient := client.Get(ctx)

		deps, err := kubeClient.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			t.Fatalf("error getting deployment: %v", err)
		}

		var dep appsv1.Deployment

		for _, d := range deps.Items {
			if kmeta.ChildName(fmt.Sprintf("apiserversource-%s-", source), string(d.GetUID())) == d.Name {
				dep = d
			}
		}

		if !MatchLabels(dep.Spec.Template.Spec.NodeSelector, TestLabels()) {
			t.Fatalf("NodeSelector labels do not match: %v", dep.Spec.Template.Spec.NodeSelector)
		}
	}
}

func SetupNodeLabels() feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		nodes, err := client.Get(ctx).CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			t.Fatalf("Could not list nodes: %v", err)
		}

		if len(nodes.Items) == 0 {
			t.Fatal("No nodes found")
		}

		node := &nodes.Items[0]

		node.Labels["testkey"] = "testvalue"
		node.Labels["testkey1"] = "testvalue1"
		node.Labels["testkey2"] = "testvalue2"

		_, err = client.Get(ctx).CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})

		if err != nil {
			t.Fatalf("Could not update node: %v", err)
		}
	}
}

func ResetNodeLabels(ctx context.Context, t feature.T) {
	nodes, err := client.Get(ctx).CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: "testkey=testvalue,testkey1=testvalue1,testkey2=testvalue2",
	})
	if err != nil {
		t.Fatalf("Could not list nodes: %v", err)
	}

	if len(nodes.Items) == 0 {
		t.Fatal("No nodes found")
	}

	node := &nodes.Items[0]

	delete(node.Labels, "testkey")
	delete(node.Labels, "testkey1")
	delete(node.Labels, "testkey2")

	_, err = client.Get(ctx).CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})

	if err != nil {
		t.Fatalf("Could not update node: %v", err)
	}
}

func WithFilters(filters []eventingv1.SubscriptionsAPIFilter) manifest.CfgFn {
	jsonBytes, err := json.Marshal(filters)
	if err != nil {
		panic(err)
	}

	yamlBytes, err := yamllib.JSONToYAML(jsonBytes)
	if err != nil {
		panic(err)
	}

	filtersYaml := string(yamlBytes)
	lines := strings.Split(filtersYaml, "\n")
	out := make([]string, 0, len(lines))
	for i := range lines {
		out = append(out, "    "+lines[i])
	}

	return func(cfg map[string]interface{}) {
		cfg["filters"] = strings.Join(out, "\n")
	}
}
