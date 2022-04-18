/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package function

import (
	"fmt"
	"strings"

	cesql "github.com/cloudevents/sdk-go/sql/v2"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

var LengthFunction function = function{
	name:         "LENGTH",
	fixedArgs:    []cesql.Type{cesql.StringType},
	variadicArgs: nil,
	fn: func(event cloudevents.Event, i []interface{}) (interface{}, error) {
		return int32(len(i[0].(string))), nil
	},
}

var ConcatFunction function = function{
	name:         "CONCAT",
	variadicArgs: cesql.TypePtr(cesql.StringType),
	fn: func(event cloudevents.Event, i []interface{}) (interface{}, error) {
		var sb strings.Builder
		for _, v := range i {
			sb.WriteString(v.(string))
		}
		return sb.String(), nil
	},
}

var ConcatWSFunction function = function{
	name:         "CONCAT_WS",
	fixedArgs:    []cesql.Type{cesql.StringType},
	variadicArgs: cesql.TypePtr(cesql.StringType),
	fn: func(event cloudevents.Event, args []interface{}) (interface{}, error) {
		if len(args) == 1 {
			return "", nil
		}
		separator := args[0].(string)

		var sb strings.Builder
		for i := 1; i < len(args)-1; i++ {
			sb.WriteString(args[i].(string))
			sb.WriteString(separator)
		}
		sb.WriteString(args[len(args)-1].(string))
		return sb.String(), nil
	},
}

var LowerFunction function = function{
	name:      "LOWER",
	fixedArgs: []cesql.Type{cesql.StringType},
	fn: func(event cloudevents.Event, i []interface{}) (interface{}, error) {
		return strings.ToLower(i[0].(string)), nil
	},
}

var UpperFunction function = function{
	name:      "UPPER",
	fixedArgs: []cesql.Type{cesql.StringType},
	fn: func(event cloudevents.Event, i []interface{}) (interface{}, error) {
		return strings.ToUpper(i[0].(string)), nil
	},
}

var TrimFunction function = function{
	name:      "TRIM",
	fixedArgs: []cesql.Type{cesql.StringType},
	fn: func(event cloudevents.Event, i []interface{}) (interface{}, error) {
		return strings.TrimSpace(i[0].(string)), nil
	},
}

var LeftFunction function = function{
	name:      "LEFT",
	fixedArgs: []cesql.Type{cesql.StringType, cesql.IntegerType},
	fn: func(event cloudevents.Event, i []interface{}) (interface{}, error) {
		str := i[0].(string)
		y := int(i[1].(int32))

		if y > len(str) {
			return str, nil
		}

		if y < 0 {
			return nil, fmt.Errorf("LEFT y argument is < 0: %d", y)
		}

		return str[0:y], nil
	},
}

var RightFunction function = function{
	name:      "RIGHT",
	fixedArgs: []cesql.Type{cesql.StringType, cesql.IntegerType},
	fn: func(event cloudevents.Event, i []interface{}) (interface{}, error) {
		str := i[0].(string)
		y := int(i[1].(int32))

		if y > len(str) {
			return str, nil
		}

		if y < 0 {
			return nil, fmt.Errorf("RIGHT y argument is < 0: %d", y)
		}

		return str[len(str)-y:], nil
	},
}

var SubstringFunction function = function{
	name:      "SUBSTRING",
	fixedArgs: []cesql.Type{cesql.StringType, cesql.IntegerType},
	fn: func(event cloudevents.Event, i []interface{}) (interface{}, error) {
		str := i[0].(string)
		pos := int(i[1].(int32))

		if pos == 0 {
			return "", nil
		}

		if pos < -len(str) || pos > len(str) {
			return "", fmt.Errorf("SUBSTRING invalid pos argument: %d", pos)
		}

		var beginning int
		if pos < 0 {
			beginning = len(str) + pos
		} else {
			beginning = pos - 1
		}

		return str[beginning:], nil
	},
}

var SubstringWithLengthFunction function = function{
	name:      "SUBSTRING",
	fixedArgs: []cesql.Type{cesql.StringType, cesql.IntegerType, cesql.IntegerType},
	fn: func(event cloudevents.Event, i []interface{}) (interface{}, error) {
		str := i[0].(string)
		pos := int(i[1].(int32))
		length := int(i[2].(int32))

		if pos == 0 {
			return "", nil
		}

		if pos < -len(str) || pos > len(str) {
			return "", fmt.Errorf("SUBSTRING invalid pos argument: %d", pos)
		}

		var beginning int
		if pos < 0 {
			beginning = len(str) + pos
		} else {
			beginning = pos - 1
		}

		var end int
		if beginning+length > len(str) {
			end = len(str)
		} else {
			end = beginning + length
		}

		return str[beginning:end], nil
	},
}
