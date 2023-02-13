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

package pod

import (
	corev1 "k8s.io/api/core/v1"
	"knative.dev/reconciler-test/pkg/manifest"
)

var WithAnnotations = manifest.WithAnnotations
var WithLabels = manifest.WithLabels

func WithEnvs(envs map[string]string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if envs != nil {
			cfg["envs"] = envs
		}
	}
}

func WithCommand(cmd []string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["command"] = cmd
	}
}

func WithArgs(args []string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["args"] = args
	}
}

func WithImagePullPolicy(ipp corev1.PullPolicy) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["imagePullPolicy"] = ipp
	}
}

func WithPort(port int) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		cfg["port"] = port
	}
}

// WithOverriddenNamespace will modify the namespace of the pod from the specs to the provided one
func WithNamespace(ns string) manifest.CfgFn {
	return func(cfg map[string]interface{}) {
		if ns != "" {
			cfg["namespace"] = ns
		}
	}
}
