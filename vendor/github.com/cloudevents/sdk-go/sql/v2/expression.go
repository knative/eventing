/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package v2

import cloudevents "github.com/cloudevents/sdk-go/v2"

// Expression represents a parsed CloudEvents SQL Expression.
type Expression interface {

	// Evaluate the expression using the provided input type.
	// The return value can be either int32, bool or string.
	// The evaluation fails as soon as an error arises.
	Evaluate(event cloudevents.Event) (interface{}, error)
}
