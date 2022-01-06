/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package utils

import (
	"fmt"
	"strconv"
	"strings"

	cesql "github.com/cloudevents/sdk-go/sql/v2"
)

func Cast(val interface{}, target cesql.Type) (interface{}, error) {
	if target.IsSameType(val) {
		return val, nil
	}
	switch target {
	case cesql.StringType:
		switch val.(type) {
		case int32:
			return strconv.Itoa(int(val.(int32))), nil
		case bool:
			if val.(bool) {
				return "true", nil
			} else {
				return "false", nil
			}
		}
		// Casting to string is always defined
		return fmt.Sprintf("%v", val), nil
	case cesql.IntegerType:
		switch val.(type) {
		case string:
			v, err := strconv.Atoi(val.(string))
			if err != nil {
				err = fmt.Errorf("cannot cast from String to Integer: %w", err)
			}
			return int32(v), err
		}
		return 0, fmt.Errorf("undefined cast from %v to %v", cesql.TypeFromVal(val), target)
	case cesql.BooleanType:
		switch val.(type) {
		case string:
			lowerCase := strings.ToLower(val.(string))
			if lowerCase == "true" {
				return true, nil
			} else if lowerCase == "false" {
				return false, nil
			}
			return false, fmt.Errorf("cannot cast String to Boolean, actual value: %v", val)
		}
		return false, fmt.Errorf("undefined cast from %v to %v", cesql.TypeFromVal(val), target)
	}

	// AnyType doesn't need casting
	return val, nil
}

func CanCast(val interface{}, target cesql.Type) bool {
	_, err := Cast(val, target)
	return err == nil
}
