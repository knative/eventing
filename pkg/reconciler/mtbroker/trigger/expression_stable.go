//+build !js_trigger_filter

package mttrigger

import (
	"context"

	"knative.dev/pkg/logging"

	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

func LogTriggerFilterExpression(ctx context.Context, trigger *v1.Trigger) {
	logging.FromContext(ctx).Info("Compiled controller doesn't contain js_trigger_filter feature")
}
