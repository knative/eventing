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

package eventcache

import (
	"context"
	"log"
	"path"
	"runtime"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/manifest"
)

func init() {
	environment.RegisterPackage(manifest.ImagesLocalYaml()...)
}

// Install
func Install(name string) feature.StepFn {
	cfg := map[string]interface{}{
		"name": name,
	}

	return func(ctx context.Context, t feature.T) {
		if _, err := manifest.InstallLocalYaml(ctx, cfg); err != nil {
			t.Fatal(err)
		}
	}
}

func IsReady(name string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		k8s.WaitForPodRunningOrFail(ctx, t, name)
		k8s.WaitForServiceEndpointsOrFail(ctx, t, name, 1)
	}
}

func PathFor(file string) string {
	_, filename, _, _ := runtime.Caller(0)
	log.Println("FILENAME: ", filename)

	return path.Join(path.Dir(filename), file)
}

// TODO: add With* methods as we need them.

// AsRef returns a KRef for a Service without namespace.
func AsRef(name string) *duckv1.KReference {
	return &duckv1.KReference{
		Kind:       "Service",
		APIVersion: "v1",
		Name:       name,
	}
}
