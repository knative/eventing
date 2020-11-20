/*
Copyright 2020 The Knative Authors

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

package environment

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/reconciler-test/pkg/feature"
)

// EnvOpts are options used to adjust the context or change how the
// environment is setup.
type EnvOpts func(ctx context.Context, env Environment) (context.Context, error)

// GlobalEnvironment is the factory for an instance of Environment.
// GlobalEnvironment holds the understanding of the particular cluster that
// will be used for the feature testing.
type GlobalEnvironment interface {
	Environment(opts ...EnvOpts) (context.Context, Environment)
}

// Environment is the ephemeral testing environment to test features.
type Environment interface {
	// Prerequisite will execute the feature using the given Context and T,
	// the feature should not have any asserts.
	Prerequisite(ctx context.Context, t *testing.T, f *feature.Feature)

	// Test will execute the feature test using the given Context and T.
	Test(ctx context.Context, t *testing.T, f *feature.Feature)

	// Namespace returns the namespace of this environment.
	Namespace() string

	// RequirementLevel returns the requirement level for this environment.
	RequirementLevel() feature.Levels
	// FeatureState returns the requirement level for this environment.
	FeatureState() feature.States

	// Images returns back the name to container image mapping to be used with
	// yaml template parsing.
	// The map will be in the form `key`: `image` and `key` and the intention
	// usage is to use this key to string substitute for image in test yaml.
	Images() map[string]string
	// TemplateConfig returns the base template config to use when processing
	// yaml templates.
	TemplateConfig(base map[string]interface{}) map[string]interface{}

	// Reference registers an object reference to the environment, so that it
	// can be listed in env.References() or be cleaned up in env.Finish().
	// This can be one way a feature communicates with future features run in
	// the same environment.
	Reference(ref ...corev1.ObjectReference)

	// References returns the list of known object references that have been
	// installed in the environment.
	References() []corev1.ObjectReference

	// Finish signals to the environment no future features will be run. The
	// namespace will be deleted if it was created by the environment,
	// References will be cleaned up if registered.
	Finish()
}
