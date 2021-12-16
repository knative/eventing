/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package function

import (
	cesql "github.com/cloudevents/sdk-go/sql/v2"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type function struct {
	name         string
	fixedArgs    []cesql.Type
	variadicArgs *cesql.Type
	fn           func(cloudevents.Event, []interface{}) (interface{}, error)
}

func (f function) Name() string {
	return f.name
}

func (f function) Arity() int {
	return len(f.fixedArgs)
}

func (f function) IsVariadic() bool {
	return f.variadicArgs != nil
}

func (f function) ArgType(index int) *cesql.Type {
	if index < len(f.fixedArgs) {
		return &f.fixedArgs[index]
	}
	return f.variadicArgs
}

func (f function) Run(event cloudevents.Event, arguments []interface{}) (interface{}, error) {
	return f.fn(event, arguments)
}
