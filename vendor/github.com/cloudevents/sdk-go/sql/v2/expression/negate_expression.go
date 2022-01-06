/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package expression

import (
	cesql "github.com/cloudevents/sdk-go/sql/v2"
	"github.com/cloudevents/sdk-go/sql/v2/utils"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type negateExpression baseUnaryExpression

func (l negateExpression) Evaluate(event cloudevents.Event) (interface{}, error) {
	val, err := l.child.Evaluate(event)
	if err != nil {
		return nil, err
	}

	val, err = utils.Cast(val, cesql.IntegerType)
	if err != nil {
		return nil, err
	}

	return -(val.(int32)), nil
}

func NewNegateExpression(child cesql.Expression) cesql.Expression {
	return negateExpression{child: child}
}
