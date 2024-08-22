/*
 * Copyright 2023 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package secret

import (
	"context"
	"embed"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeclient "knative.dev/pkg/client/injection/kube/client"

	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed *.yaml
var yaml embed.FS

func Install(name string, options ...manifest.CfgFn) feature.StepFn {
	cfg := map[string]interface{}{
		"name": name,
	}

	for _, fn := range options {
		fn(cfg)
	}

	return func(ctx context.Context, t feature.T) {
		if _, err := manifest.InstallYamlFS(ctx, yaml, cfg); err != nil {
			t.Fatal(err)
		}
	}
}

type Assertion func(s *corev1.Secret) error

func IsPresent(name string, assertions ...Assertion) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		IsPresentInNamespace(name, environment.FromContext(ctx).Namespace(), assertions...)(ctx, t)
	}
}

func IsPresentInNamespace(name string, ns string, assertions ...Assertion) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		interval, timeout := environment.PollTimingsFromContext(ctx)

		var lastErr error
		err := wait.PollImmediate(interval, timeout, func() (bool, error) {
			_, err := kubeclient.Get(ctx).CoreV1().
				Secrets(ns).
				Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				lastErr = err
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			t.Errorf("failed to get secret %s/%s: %v", ns, name, lastErr)
			return
		}

		secret, err := kubeclient.Get(ctx).CoreV1().
			Secrets(ns).
			Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			t.Error(err)
			return
		}

		for _, assertion := range assertions {
			if err := assertion(secret); err != nil {
				t.Error(err)
			}
		}
	}
}

func AssertKey(key string) Assertion {
	return func(s *corev1.Secret) error {
		_, ok := s.Data[key]
		if !ok {
			return fmt.Errorf("failed to find key %s in secret %s/%s", key, s.Namespace, s.Name)
		}
		return nil
	}
}
