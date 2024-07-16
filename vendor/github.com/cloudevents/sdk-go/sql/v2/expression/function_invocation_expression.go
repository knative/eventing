/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package expression

import (
	"fmt"

	cesql "github.com/cloudevents/sdk-go/sql/v2"
	sqlerrors "github.com/cloudevents/sdk-go/sql/v2/errors"
	"github.com/cloudevents/sdk-go/sql/v2/runtime"
	"github.com/cloudevents/sdk-go/sql/v2/utils"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type functionInvocationExpression struct {
	name                string
	argumentsExpression []cesql.Expression
}

func (expr functionInvocationExpression) Evaluate(event cloudevents.Event) (interface{}, error) {
	fn := runtime.ResolveFunction(expr.name, len(expr.argumentsExpression))
	if fn == nil {
		return false, sqlerrors.NewMissingFunctionError(expr.name)
	}

	args := make([]interface{}, len(expr.argumentsExpression))

	defaultVal := fn.ReturnType().ZeroValue()

	for i, expr := range expr.argumentsExpression {
		arg, err := expr.Evaluate(event)
		if err != nil {
			return defaultVal, err
		}

		argType := fn.ArgType(i)
		if argType == nil {
			return defaultVal, sqlerrors.NewFunctionEvaluationError(fmt.Errorf("cannot resolve arg type at index %d for function %s", i, fn.Name()))
		}

		arg, err = utils.Cast(arg, *argType)
		if err != nil {
			return defaultVal, err
		}

		args[i] = arg
	}

	result, err := fn.Run(event, args)
	if result == nil {
		if err != nil {
			err = sqlerrors.NewFunctionEvaluationError(fmt.Errorf("function %s encountered error %w and did not return any value, defaulting to the default value for the function", fn.Name(), err))
		} else {
			err = sqlerrors.NewFunctionEvaluationError(fmt.Errorf("function %s did not return any value, defaulting to the default value for the function", fn.Name()))
		}

		return defaultVal, err
	}

	return result, err
}

func NewFunctionInvocationExpression(name string, argumentsExpression []cesql.Expression) cesql.Expression {
	return functionInvocationExpression{
		name:                name,
		argumentsExpression: argumentsExpression,
	}
}
