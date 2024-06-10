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

package jobsink

import (
	"context"
	"embed"
	"encoding/json"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/ptr"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/knative"
	"knative.dev/reconciler-test/pkg/manifest"
	"sigs.k8s.io/yaml"

	"knative.dev/eventing/test/rekt/resources/addressable"
)

//go:embed jobsink.yaml
var yamlEmbed embed.FS

func GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "sinks.knative.dev", Version: "v1alpha1", Resource: "jobsinks"}
}

// WithAnnotations adds annotations to the JobSink.
func WithAnnotations(annotations map[string]interface{}) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if annotations != nil {
			cfg["annotations"] = annotations
		}
	}
}

// Install will create a resource, augmented with the config fn options.
func Install(name string, opts ...manifest.CfgFn) feature.StepFn {

	return func(ctx context.Context, t feature.T) {
		cfg := map[string]interface{}{
			"name":                     name,
			"namespace":                environment.FromContext(ctx).Namespace(),
			"image":                    eventshub.ImageFromContext(ctx),
			eventshub.ConfigLoggingEnv: knative.LoggingConfigFromContext(ctx),
			eventshub.ConfigTracingEnv: knative.TracingConfigFromContext(ctx),
		}
		for _, fn := range opts {
			fn(cfg)
		}

		if _, err := manifest.InstallYamlFS(ctx, yamlEmbed, cfg); err != nil {
			t.Fatal(err)
		}
	}
}

func WithJob(job batchv1.Job) manifest.CfgFn {
	jsonBytes, err := json.Marshal(job)
	if err != nil {
		panic(err)
	}

	yamlBytes, err := yaml.JSONToYAML(jsonBytes)
	if err != nil {
		panic(err)
	}

	jobYaml := string(yamlBytes)

	lines := strings.Split(jobYaml, "\n")
	out := make([]string, 0, len(lines))
	for i := range lines {
		out = append(out, "    "+lines[i])
	}

	return func(m map[string]interface{}) {
		m["job"] = strings.Join(out, "\n")
	}
}

func WithForwarderJob(sink string, options ...func(*batchv1.Job)) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		j := batchv1.Job{
			Spec: batchv1.JobSpec{
				Parallelism:  ptr.Int32(1),
				Completions:  ptr.Int32(1),
				BackoffLimit: ptr.Int32(10),
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyOnFailure,
						Containers: []corev1.Container{
							{
								Name:  "forwarder",
								Image: cfg["image"].(string),
								Env: []corev1.EnvVar{
									{
										Name:  "EVENT_GENERATORS",
										Value: "forwarder",
									},
									{
										Name:  "EVENT_LOGS",
										Value: "logger",
									},
									{
										Name:  "NAME",
										Value: cfg["name"].(string),
									},
									{
										Name:  "NAMESPACE",
										Value: cfg["namespace"].(string),
									},
									{
										Name:  "SINK",
										Value: sink,
									},
									{
										Name:  "FROM_FILES",
										Value: "/etc/jobsink-event/event",
									},
									{
										Name:  eventshub.ConfigLoggingEnv,
										Value: cfg[eventshub.ConfigLoggingEnv].(string),
									},
									{
										Name:  eventshub.ConfigTracingEnv,
										Value: cfg[eventshub.ConfigTracingEnv].(string),
									},
								},
							},
						},
					},
				},
			},
		}

		for _, opt := range options {
			opt(&j)
		}

		WithJob(j)(cfg)
	}
}

// IsReady tests to see if a JobSink becomes ready within the time given.
func IsReady(name string, timing ...time.Duration) feature.StepFn {
	return k8s.IsReady(GVR(), name, timing...)
}

// IsNotReady tests to see if a JobSink becomes NotReady within the time given.
func IsNotReady(name string, timing ...time.Duration) feature.StepFn {
	return k8s.IsNotReady(GVR(), name, timing...)
}

// IsAddressable tests to see if a JobSink becomes addressable within the  time
// given.
func IsAddressable(name string, timings ...time.Duration) feature.StepFn {
	return k8s.IsAddressable(GVR(), name, timings...)
}

// ValidateAddress validates the address retured by Address
func ValidateAddress(name string, validate addressable.ValidateAddressFn, timings ...time.Duration) feature.StepFn {
	return addressable.ValidateAddress(GVR(), name, validate, timings...)
}

// Address returns a JobSink's address.
func Address(ctx context.Context, name string, timings ...time.Duration) (*duckv1.Addressable, error) {
	return addressable.Address(ctx, GVR(), name, timings...)
}

func AsDestinationRef(name string) *duckv1.Destination {
	return &duckv1.Destination{
		Ref: AsKReference(name),
	}
}

// AsKReference returns a KReference for a JobSink without namespace.
func AsKReference(name string) *duckv1.KReference {
	return &duckv1.KReference{
		Kind:       "JobSink",
		Name:       name,
		APIVersion: GVR().GroupVersion().String(),
	}
}
