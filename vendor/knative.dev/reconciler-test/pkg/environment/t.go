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
	"testing"

	"go.uber.org/atomic"

	"knative.dev/reconciler-test/pkg/feature"
)

// skippingT records if the test succeeded or failing, but never fails it
type skippingT struct {
	*testing.T

	failed  *atomic.Bool
	skipped *atomic.Bool
}

var _ feature.T = (*skippingT)(nil)

func createSkippingT(originalT *testing.T) feature.T {
	return &skippingT{
		T:       originalT,
		failed:  atomic.NewBool(false),
		skipped: atomic.NewBool(false),
	}
}

func (t *skippingT) Error(args ...interface{}) {
	t.T.Helper()
	t.failed.Store(true)
	t.T.Log(args...)
}

func (t *skippingT) Errorf(format string, args ...interface{}) {
	t.T.Helper()
	t.failed.Store(true)
	t.T.Logf(format, args...)
}

func (t *skippingT) Fail() {
	t.T.Helper()
	t.failed.Store(true)
}

func (t *skippingT) FailNow() {
	t.T.Helper()
	t.failed.Store(true)
	t.T.SkipNow()
}

func (t *skippingT) Failed() bool {
	return t.failed.Load()
}

func (t *skippingT) Fatal(args ...interface{}) {
	t.T.Helper()
	t.failed.Store(true)
	t.T.Log(args...)
	t.T.SkipNow()
}

func (t *skippingT) Fatalf(format string, args ...interface{}) {
	t.T.Helper()
	t.failed.Store(true)
	t.T.Logf(format, args...)
	t.T.SkipNow()
}

func (t *skippingT) Log(args ...interface{}) {
	t.T.Helper()
	t.T.Log(args...)
}

func (t *skippingT) Logf(format string, args ...interface{}) {
	t.T.Helper()
	t.T.Logf(format, args...)
}

func (t *skippingT) Skip(args ...interface{}) {
	t.T.Helper()
	t.skipped.Store(true)
	t.T.Log(args...)
	t.T.SkipNow()
}

func (t *skippingT) Skipf(format string, args ...interface{}) {
	t.T.Helper()
	t.skipped.Store(true)
	t.T.Logf(format, args...)
	t.T.SkipNow()
}

func (t *skippingT) SkipNow() {
	t.T.Helper()
	t.skipped.Store(true)
	t.T.SkipNow()
}

func (t *skippingT) Skipped() bool {
	return t.skipped.Load()
}
