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

package sut

import (
	"context"

	"go.uber.org/zap"
	testlib "knative.dev/eventing/test/lib"
	watholaevent "knative.dev/eventing/test/upgrade/prober/wathola/event"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var eventTypes = []string{
	watholaevent.Step{}.Type(),
	watholaevent.Finished{}.Type(),
}

// SystemUnderTest (SUT) represents a system that we'd like to test with
// continual prober.
type SystemUnderTest interface {
	// Deploy is responsible for deploying SUT and returning a URL to feed
	// events into.
	Deploy(ctx Context, destination duckv1.Destination) interface{}
}

// HasTeardown indicates that SystemUnderTest supports custom teardown that
// exceeds regular teardown via usage of testlib.Tracker.
type HasTeardown interface {
	// Teardown will remove all deployed SUT resources.
	Teardown(ctx Context)
}

// Context represents a context of system under test that we'd
// like to deploy and teardown.
type Context struct {
	Ctx context.Context
	Log *zap.SugaredLogger
	*testlib.Client
}
