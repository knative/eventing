/*
Copyright 2022 The Knative Authors

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

package manifest

import (
	"context"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	pkgsecurity "knative.dev/pkg/test/security"

	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
)

// PodSecurityCfgFn returns a function for configuring security context for Pod, depending
// on security settings of the enclosing namespace.
func PodSecurityCfgFn(ctx context.Context, t feature.T) CfgFn {
	namespace := environment.FromContext(ctx).Namespace()
	restrictedMode, err := pkgsecurity.IsRestrictedPodSecurityEnforced(ctx, kubeclient.Get(ctx), namespace)
	if err != nil {
		t.Fatalf("Error while checking restricted pod security mode for namespace %s", namespace)
	}
	if restrictedMode {
		return k8s.WithDefaultPodSecurityContext
	}
	return func(map[string]interface{}) {}
}

// WithAnnotations returns a function for configuring annototations of the resource
func WithAnnotations(annotations map[string]interface{}) CfgFn {
	return func(cfg map[string]interface{}) {
		if original, ok := cfg["annotations"]; ok {
			appendToOriginal(original, annotations)
			return
		}
		cfg["annotations"] = annotations
	}
}

// WithPodAnnotations appends pod annotations (usually used by types where pod template is embedded)
func WithPodAnnotations(additional map[string]interface{}) CfgFn {
	return func(cfg map[string]interface{}) {
		if ann, ok := cfg["podannotations"]; ok {
			appendToOriginal(ann, additional)
			return
		}
		cfg["podannotations"] = additional
	}
}

// WithPodLabels appends pod labels (usually used by types where pod template is embedded)
func WithPodLabels(additional map[string]string) CfgFn {
	return func(cfg map[string]interface{}) {
		if ann, ok := cfg["podlabels"]; ok {
			m := make(map[string]interface{}, len(additional))
			for k, v := range additional {
				m[k] = v
			}
			appendToOriginal(ann, m)
			return
		}
		cfg["podlabels"] = additional
	}
}

func appendToOriginal(original interface{}, additional map[string]interface{}) {
	annotations := original.(map[string]interface{})
	for k, v := range additional {
		// Only add the unspecified ones
		if _, ok := annotations[k]; !ok {
			annotations[k] = v
		}
	}
}

// WithLabels returns a function for configuring labels of the resource
func WithLabels(labels map[string]string) CfgFn {
	return func(cfg map[string]interface{}) {
		if labels != nil {
			cfg["labels"] = labels
		}
	}
}

func WithIstioPodAnnotations(cfg map[string]interface{}) {
	podAnnotations := map[string]interface{}{
		"sidecar.istio.io/inject":                "true",
		"sidecar.istio.io/rewriteAppHTTPProbers": "true",
		"proxy.istio.io/config":                  "{ 'holdApplicationUntilProxyStarts': true }",
	}

	WithAnnotations(podAnnotations)(cfg)
	WithPodAnnotations(podAnnotations)(cfg)
}

func WithIstioPodLabels(cfg map[string]interface{}) {
	podLabels := map[string]string{
		"sidecar.istio.io/inject": "true",
	}

	WithLabels(podLabels)(cfg)
	WithPodLabels(podLabels)(cfg)
}
