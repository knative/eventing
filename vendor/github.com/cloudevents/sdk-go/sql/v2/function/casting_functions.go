/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package function

import (
	cesql "github.com/cloudevents/sdk-go/sql/v2"
	"github.com/cloudevents/sdk-go/sql/v2/utils"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

var IntFunction function = function{
	name:         "INT",
	fixedArgs:    []cesql.Type{cesql.AnyType},
	variadicArgs: nil,
	fn: func(event cloudevents.Event, i []interface{}) (interface{}, error) {
		return utils.Cast(i[0], cesql.IntegerType)
	},
}

var BoolFunction function = function{
	name:         "BOOL",
	fixedArgs:    []cesql.Type{cesql.AnyType},
	variadicArgs: nil,
	fn: func(event cloudevents.Event, i []interface{}) (interface{}, error) {
		return utils.Cast(i[0], cesql.BooleanType)
	},
}

var StringFunction function = function{
	name:         "STRING",
	fixedArgs:    []cesql.Type{cesql.AnyType},
	variadicArgs: nil,
	fn: func(event cloudevents.Event, i []interface{}) (interface{}, error) {
		return utils.Cast(i[0], cesql.StringType)
	},
}

var IsIntFunction function = function{
	name:         "IS_INT",
	fixedArgs:    []cesql.Type{cesql.AnyType},
	variadicArgs: nil,
	fn: func(event cloudevents.Event, i []interface{}) (interface{}, error) {
		return utils.CanCast(i[0], cesql.IntegerType), nil
	},
}

var IsBoolFunction function = function{
	name:         "IS_BOOL",
	fixedArgs:    []cesql.Type{cesql.AnyType},
	variadicArgs: nil,
	fn: func(event cloudevents.Event, i []interface{}) (interface{}, error) {
		return utils.CanCast(i[0], cesql.BooleanType), nil
	},
}
