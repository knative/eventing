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
