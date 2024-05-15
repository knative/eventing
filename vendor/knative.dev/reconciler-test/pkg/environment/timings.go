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

package environment

import (
	"context"
	"time"

	"knative.dev/reconciler-test/pkg/state"
)

// this has been moved to state pkg to break cycle between environment and feature package,
// keeping the consts here for backwards API compatibility
const (
	DefaultPollInterval = 1 * time.Second
	DefaultPollTimeout  = 2 * time.Minute
)

// ContextWithPollTimings returns a context with poll timings set
func ContextWithPollTimings(ctx context.Context, interval, timeout time.Duration) context.Context {
	return state.ContextWithPollTimings(ctx, interval, timeout)
}

// PollTimingsFromContext will get the previously set poll timing from context,
// or return the defaults if not found.
// - values from context.
// - defaults.
func PollTimingsFromContext(ctx context.Context) (time.Duration, time.Duration) {
	return state.PollTimingsFromContext(ctx)
}
