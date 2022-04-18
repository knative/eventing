/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package expression

import (
	cesql "github.com/cloudevents/sdk-go/sql/v2"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type literalExpression struct {
	value interface{}
}

func (l literalExpression) Evaluate(event cloudevents.Event) (interface{}, error) {
	return l.value, nil
}

func NewLiteralExpression(value interface{}) cesql.Expression {
	return literalExpression{value: value}
}
