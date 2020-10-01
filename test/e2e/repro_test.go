package e2e

import (
	"context"
	"testing"

	"knative.dev/eventing/test/e2e/helpers"
)

func TestRepro(t *testing.T) {
	helpers.Repro(context.Background(), t, channelTestRunner)
}
