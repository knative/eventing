/*
Copyright 2022 The Knative Authors

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

package subscriptionsapi

import (
	"context"
	"fmt"

	cesql "github.com/cloudevents/sdk-go/sql/v2"
	cesqlparser "github.com/cloudevents/sdk-go/sql/v2/parser"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/eventfilter"
)

type ceSQLFilter struct {
	rawExpression    string
	parsedExpression cesql.Expression
}

// NewCESQLFilter returns an event filter which passes if the provided CESQL expression
// evaluates.
func NewCESQLFilter(expr string) (eventfilter.Filter, error) {
	var parsed cesql.Expression
	var err error
	if expr != "" {
		parsed, err = cesqlparser.Parse(expr)
		if err != nil {
			return nil, fmt.Errorf("error while parsing expression %s. Error: %w", expr, err)
		}
	}
	return &ceSQLFilter{
		rawExpression:    expr,
		parsedExpression: parsed,
	}, nil
}

func (filter *ceSQLFilter) Filter(ctx context.Context, event cloudevents.Event) eventfilter.FilterResult {
	if filter == nil || filter.rawExpression == "" {
		return eventfilter.NoFilter
	}
	logger := logging.FromContext(ctx)
	logger.Debugw("Performing a CESQL match ", zap.String("expression", filter.rawExpression), zap.Any("event", event))

	res, err := filter.parsedExpression.Evaluate(event)

	if err != nil {
		logger.Debugw("Error evaluating expression on event.", zap.String("expression", filter.rawExpression), zap.Error(err))
		return eventfilter.FailFilter
	}

	if !res.(bool) {
		logger.Debugw("CESOL match failed.", zap.String("expression", filter.rawExpression), zap.Any("event", event))
		return eventfilter.FailFilter
	}
	return eventfilter.PassFilter
}

func (filter *ceSQLFilter) Cleanup() {}
