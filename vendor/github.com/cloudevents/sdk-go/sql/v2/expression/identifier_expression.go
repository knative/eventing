/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package expression

import (
	cesql "github.com/cloudevents/sdk-go/sql/v2"
	sqlerrors "github.com/cloudevents/sdk-go/sql/v2/errors"
	"github.com/cloudevents/sdk-go/sql/v2/utils"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type identifierExpression struct {
	identifier string
}

func (l identifierExpression) Evaluate(event cloudevents.Event) (interface{}, error) {
	value := utils.GetAttribute(event, l.identifier)
	if value == nil {
		return false, sqlerrors.NewMissingAttributeError(l.identifier)
	}

	return value, nil
}

func NewIdentifierExpression(identifier string) cesql.Expression {
	return identifierExpression{identifier: identifier}
}
