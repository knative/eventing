package environment

import (
	"testing"

	"go.uber.org/atomic"

	"knative.dev/reconciler-test/pkg/feature"
)

// requirementT records if the test succeeded or failing, but never fails it in order to avoid bad reporting
type requirementT struct {
	*testing.T

	failed  *atomic.Bool
	skipped *atomic.Bool
}

var _ feature.T = (*requirementT)(nil)

func createRequirementT(originalT *testing.T) *requirementT {
	return &requirementT{
		T:       originalT,
		failed:  atomic.NewBool(false),
		skipped: atomic.NewBool(false),
	}
}

func (t *requirementT) Error(args ...interface{}) {
	t.T.Helper()
	t.failed.Store(true)
	t.T.Log(args...)
}

func (t *requirementT) Errorf(format string, args ...interface{}) {
	t.T.Helper()
	t.failed.Store(true)
	t.T.Logf(format, args...)
}

func (t *requirementT) Fail() {
	t.T.Helper()
	t.failed.Store(true)
}

func (t *requirementT) FailNow() {
	t.T.Helper()
	t.failed.Store(true)
	t.T.SkipNow()
}

func (t *requirementT) Failed() bool {
	return t.failed.Load()
}

func (t *requirementT) Fatal(args ...interface{}) {
	t.T.Helper()
	t.failed.Store(true)
	t.T.Log(args...)
	t.T.SkipNow()
}

func (t *requirementT) Fatalf(format string, args ...interface{}) {
	t.T.Helper()
	t.failed.Store(true)
	t.T.Logf(format, args...)
	t.T.SkipNow()
}

func (t *requirementT) Log(args ...interface{}) {
	t.T.Helper()
	t.T.Log(args...)
}

func (t *requirementT) Logf(format string, args ...interface{}) {
	t.T.Helper()
	t.T.Logf(format, args...)
}

func (t *requirementT) Skip(args ...interface{}) {
	t.T.Helper()
	t.skipped.Store(true)
	t.T.Log(args...)
	t.T.SkipNow()
}

func (t *requirementT) Skipf(format string, args ...interface{}) {
	t.T.Helper()
	t.skipped.Store(true)
	t.T.Logf(format, args...)
	t.T.SkipNow()
}

func (t *requirementT) SkipNow() {
	t.T.Helper()
	t.skipped.Store(true)
	t.T.SkipNow()
}

func (t *requirementT) Skipped() bool {
	return t.skipped.Load()
}
