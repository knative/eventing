/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package expression

import (
	"fmt"

	cesql "github.com/cloudevents/sdk-go/sql/v2"
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
		return nil, fmt.Errorf("cannot resolve function %s", expr.name)
	}

	args := make([]interface{}, len(expr.argumentsExpression))

	for i, expr := range expr.argumentsExpression {
		arg, err := expr.Evaluate(event)
		if err != nil {
			return nil, err
		}

		argType := fn.ArgType(i)
		if argType == nil {
			return nil, fmt.Errorf("cannot resolve arg type at index %d", i)
		}

		arg, err = utils.Cast(arg, *argType)
		if err != nil {
			return nil, err
		}

		args[i] = arg
	}

	return fn.Run(event, args)
}

func NewFunctionInvocationExpression(name string, argumentsExpression []cesql.Expression) cesql.Expression {
	return functionInvocationExpression{
		name:                name,
		argumentsExpression: argumentsExpression,
	}
}
