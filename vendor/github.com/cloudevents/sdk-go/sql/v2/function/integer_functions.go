/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package function

import (
	cesql "github.com/cloudevents/sdk-go/sql/v2"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

var AbsFunction function = function{
	name:         "ABS",
	fixedArgs:    []cesql.Type{cesql.IntegerType},
	variadicArgs: nil,
	fn: func(event cloudevents.Event, i []interface{}) (interface{}, error) {
		x := i[0].(int32)
		if x < 0 {
			return -x, nil
		}
		return x, nil
	},
}
