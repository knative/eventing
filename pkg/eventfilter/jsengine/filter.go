package jsengine

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/dop251/goja"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/eventfilter"
)

type jsFilter goja.Program

func (j *jsFilter) Filter(ctx context.Context, event cloudevents.Event) eventfilter.FilterResult {
	pass, err := runFilter(event, (*goja.Program)(j))
	if err != nil {
		logging.FromContext(ctx).Warn("Error while trying to run the js expression filter: ", err)
		return eventfilter.FailFilter
	}
	if pass {
		return eventfilter.PassFilter
	} else {
		return eventfilter.FailFilter
	}
}

func NewJsFilter(src string) (eventfilter.Filter, error) {
	p, err := ParseFilterExpr(src)
	if err != nil {
		return nil, err
	}
	return (*jsFilter)(p), nil
}
