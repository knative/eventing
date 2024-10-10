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

package job

import (
	corev1 "k8s.io/api/core/v1"

	"knative.dev/reconciler-test/pkg/manifest"
)

var (
	WithLabels         = manifest.WithLabels
	WithPodLabels      = manifest.WithPodLabels
	WithAnnotations    = manifest.WithAnnotations
	WithPodAnnotations = manifest.WithPodAnnotations
)

func WithEnvs(envs map[string]string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if envs != nil {
			cfg["envs"] = envs
		}
	}
}

func WithImagePullPolicy(ipp corev1.PullPolicy) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["imagePullPolicy"] = ipp
	}
}

func WithRestartPolicy(restartPolicy corev1.RestartPolicy) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["restartPolicy"] = restartPolicy
	}
}

func WithBackoffLimit(backoffLimit int) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["backoffLimit"] = backoffLimit
	}
}

func WithTTLSecondsAfterFinished(ttlSecondsAfterFinished int) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["ttlSecondsAfterFinished"] = ttlSecondsAfterFinished
	}
}
