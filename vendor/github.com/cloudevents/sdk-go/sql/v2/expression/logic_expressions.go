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

type logicExpression struct {
	baseBinaryExpression
	fn   func(x, y bool) bool
	verb string
}

func (s logicExpression) Evaluate(event cloudevents.Event) (interface{}, error) {
	leftVal, err := s.left.Evaluate(event)
	if err != nil {
		return false, err
	}

	leftVal, err = utils.Cast(leftVal, cesql.BooleanType)
	if err != nil {
		return false, err
	}

	// Don't bother to check the other expression unless we need to
	if s.verb == "AND" && leftVal.(bool) == false {
		return false, nil
	}
	if s.verb == "OR" && leftVal.(bool) == true {
		return true, nil
	}

	rightVal, err := s.right.Evaluate(event)
	if err != nil {
		return false, err
	}

	rightVal, err = utils.Cast(rightVal, cesql.BooleanType)
	if err != nil {
		return false, err
	}

	return s.fn(leftVal.(bool), rightVal.(bool)), nil
}

func NewAndExpression(left cesql.Expression, right cesql.Expression) cesql.Expression {
	return logicExpression{
		baseBinaryExpression: baseBinaryExpression{
			left:  left,
			right: right,
		},
		fn: func(x, y bool) bool {
			return x && y
		},
		verb: "AND",
	}
}

func NewOrExpression(left cesql.Expression, right cesql.Expression) cesql.Expression {
	return logicExpression{
		baseBinaryExpression: baseBinaryExpression{
			left:  left,
			right: right,
		},
		fn: func(x, y bool) bool {
			return x || y
		},
		verb: "OR",
	}
}

func NewXorExpression(left cesql.Expression, right cesql.Expression) cesql.Expression {
	return logicExpression{
		baseBinaryExpression: baseBinaryExpression{
			left:  left,
			right: right,
		},
		fn: func(x, y bool) bool {
			return x != y
		},
		verb: "XOR",
	}
}
