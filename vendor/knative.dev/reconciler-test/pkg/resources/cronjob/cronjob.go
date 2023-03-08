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

package cronjob

import (
	"context"
	"embed"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeclient "knative.dev/pkg/client/injection/kube/client"

	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"
	"knative.dev/reconciler-test/pkg/manifest"
)

//go:embed *.yaml
var yaml embed.FS

func Install(name string, image string, options ...manifest.CfgFn) feature.StepFn {
	cfg := map[string]interface{}{
		"name":  name,
		"image": image,
	}

	defaultOptions := []manifest.CfgFn{
		WithRestartPolicy(corev1.RestartPolicyOnFailure),
	}

	options = append(defaultOptions, options...)

	for _, fn := range options {
		fn(cfg)
	}

	return func(ctx context.Context, t feature.T) {
		if err := registerImage(ctx, image); err != nil {
			t.Fatal(err)
		}

		if ic := environment.GetIstioConfig(ctx); ic.Enabled {
			manifest.WithIstioPodAnnotations(cfg)
		}

		manifest.PodSecurityCfgFn(ctx, t)(cfg)

		if _, err := manifest.InstallYamlFS(ctx, yaml, cfg); err != nil {
			t.Fatal(err)
		}
	}
}

func getJobNameFromCronJobName(ctx context.Context, t feature.T, name string) string {
	var job *batchv1.Job
	selector, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": name,
		},
	})

	interval, timeout := environment.PollTimingsFromContext(ctx)
	err := wait.PollImmediate(interval, timeout, func() (done bool, err error) {

		jobs, err := kubeclient.Get(ctx).BatchV1().
			Jobs(environment.FromContext(ctx).Namespace()).
			List(ctx, metav1.ListOptions{
				LabelSelector: selector.String(),
			})
		if err != nil {
			t.Error(err)
			return false, err
		}

		for _, j := range jobs.Items {
			job = &j
			return true, nil
		}

		t.Logf("No jobs found for selector %s", selector.String())

		return false, nil
	})
	if err != nil {
		t.Errorf("Failed to find a job matching selector %s", selector)
		return ""
	}

	return job.Name
}

func AtLeastOneIsDone(name string, timing ...time.Duration) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		if err := k8s.WaitUntilJobDone(ctx, t, getJobNameFromCronJobName(ctx, t, name), timing...); err != nil {
			t.Error("Job did not turn into done state", err)
		}
	}
}

func AtLeastOneIsSucceeded(name string, timing ...time.Duration) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		if err := k8s.WaitUntilJobSucceeded(ctx, t, getJobNameFromCronJobName(ctx, t, name), timing...); err != nil {
			t.Error("Job did not succeed", err)
		}
	}
}

func AtLeastOneIsFailed(name string, timing ...time.Duration) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		if err := k8s.WaitUntilJobFailed(ctx, t, getJobNameFromCronJobName(ctx, t, name), timing...); err != nil {
			t.Error("Job did not fail", err)
		}
	}
}

func registerImage(ctx context.Context, image string) error {
	reg := environment.RegisterPackage(image)
	_, err := reg(ctx, environment.FromContext(ctx))
	return err
}
