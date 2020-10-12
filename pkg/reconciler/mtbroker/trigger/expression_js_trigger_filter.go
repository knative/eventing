//+build js_trigger_filter

package mttrigger

import (
	"context"

	"knative.dev/pkg/logging"

	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1experimental"
)

func LogTriggerFilterExpression(ctx context.Context, trigger *v1.Trigger) {
	triggerExperimental := &v1experimental.Trigger{}

	err := triggerExperimental.ConvertFrom(ctx, trigger)
	if err != nil {
		logging.FromContext(ctx).Error(err)
		return
	}
	logging.FromContext(ctx).Infof("Expression: %s", triggerExperimental.Spec.Filter.Expression)
}
