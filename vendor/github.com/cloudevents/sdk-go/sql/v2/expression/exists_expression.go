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

type existsExpression struct {
	identifier string
}

func (l existsExpression) Evaluate(event cloudevents.Event) (interface{}, error) {
	return utils.ContainsAttribute(event, l.identifier), nil
}

func NewExistsExpression(identifier string) cesql.Expression {
	return existsExpression{identifier: identifier}
}
