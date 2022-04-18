/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package v2

import cloudevents "github.com/cloudevents/sdk-go/v2"

type Function interface {
	Name() string
	Arity() int
	IsVariadic() bool
	ArgType(index int) *Type

	Run(event cloudevents.Event, arguments []interface{}) (interface{}, error)
}
