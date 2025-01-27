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

package v1alpha1

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

func (sink *JobSink) SetDefaults(ctx context.Context) {
	if sink.Spec.Job != nil {
		setBatchJobDefaults(sink.Spec.Job)
	}
}

func setBatchJobDefaults(job *batchv1.Job) {
	for i := range job.Spec.Template.Spec.Containers {
		executionModeFound := false
		for j := range job.Spec.Template.Spec.Containers[i].Env {
			if job.Spec.Template.Spec.Containers[i].Env[j].Name == ExecutionModeEnvVar {
				executionModeFound = true
				break
			}
		}
		if executionModeFound {
			continue
		}
		job.Spec.Template.Spec.Containers[i].Env = append(job.Spec.Template.Spec.Containers[i].Env, corev1.EnvVar{
			Name:  ExecutionModeEnvVar,
			Value: string(ExecutionModeBatch),
		})
	}
	for i := range job.Spec.Template.Spec.InitContainers {
		executionModeFound := false
		for j := range job.Spec.Template.Spec.InitContainers[i].Env {
			if job.Spec.Template.Spec.InitContainers[i].Env[j].Name == ExecutionModeEnvVar {
				executionModeFound = true
				break
			}
		}
		if executionModeFound {
			continue
		}
		job.Spec.Template.Spec.InitContainers[i].Env = append(job.Spec.Template.Spec.InitContainers[i].Env, corev1.EnvVar{
			Name:  ExecutionModeEnvVar,
			Value: string(ExecutionModeBatch),
		})
	}
}
