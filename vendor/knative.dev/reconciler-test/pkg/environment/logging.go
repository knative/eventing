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

	"go.uber.org/zap/zaptest"

	"knative.dev/reconciler-test/pkg/feature"
	testlog "knative.dev/reconciler-test/pkg/logging"
)

// loggingSteps returns a number of steps that logs environment-managed resources.
func (mr *MagicEnvironment) loggingSteps() []feature.Step {
	mr.refsMu.Lock()
	defer mr.refsMu.Unlock()

	return []feature.Step{{
		Name: "Log references",
		S:    feature.Any,
		L:    feature.Must,
		T:    feature.Teardown,
		Fn:   feature.LogReferences(mr.refs...),
	}}
}

// WithTestLogger returns a context with test logger configured.
func WithTestLogger(t zaptest.TestingT, opts ...zaptest.LoggerOption) EnvOpts {
	return func(ctx context.Context, env Environment) (context.Context, error) {
		return testlog.WithTestLogger(ctx, t, opts...), nil
	}
}
