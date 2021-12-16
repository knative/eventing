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

type integerComparisonExpression struct {
	baseBinaryExpression
	fn func(x, y int32) bool
}

func (s integerComparisonExpression) Evaluate(event cloudevents.Event) (interface{}, error) {
	leftVal, err := s.left.Evaluate(event)
	if err != nil {
		return nil, err
	}

	rightVal, err := s.right.Evaluate(event)
	if err != nil {
		return nil, err
	}

	leftVal, err = utils.Cast(leftVal, cesql.IntegerType)
	if err != nil {
		return nil, err
	}

	rightVal, err = utils.Cast(rightVal, cesql.IntegerType)
	if err != nil {
		return nil, err
	}

	return s.fn(leftVal.(int32), rightVal.(int32)), nil
}

func NewLessExpression(left cesql.Expression, right cesql.Expression) cesql.Expression {
	return integerComparisonExpression{
		baseBinaryExpression: baseBinaryExpression{
			left:  left,
			right: right,
		},
		fn: func(x, y int32) bool {
			return x < y
		},
	}
}

func NewLessOrEqualExpression(left cesql.Expression, right cesql.Expression) cesql.Expression {
	return integerComparisonExpression{
		baseBinaryExpression: baseBinaryExpression{
			left:  left,
			right: right,
		},
		fn: func(x, y int32) bool {
			return x <= y
		},
	}
}

func NewGreaterExpression(left cesql.Expression, right cesql.Expression) cesql.Expression {
	return integerComparisonExpression{
		baseBinaryExpression: baseBinaryExpression{
			left:  left,
			right: right,
		},
		fn: func(x, y int32) bool {
			return x > y
		},
	}
}

func NewGreaterOrEqualExpression(left cesql.Expression, right cesql.Expression) cesql.Expression {
	return integerComparisonExpression{
		baseBinaryExpression: baseBinaryExpression{
			left:  left,
			right: right,
		},
		fn: func(x, y int32) bool {
			return x >= y
		},
	}
}
