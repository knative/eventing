/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package utils

import (
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
	"github.com/cloudevents/sdk-go/v2/types"
)

func GetAttribute(event cloudevents.Event, attributeName string) interface{} {
	var val interface{}

	if a := spec.V1.Attribute(attributeName); a != nil { // Standard attribute
		val = a.Get(event.Context)
	} else {
		val = event.Extensions()[attributeName]
	}

	if val == nil {
		return nil
	}

	// Type cohercion
	switch val.(type) {
	case bool, int32, string:
		return val
	case int8:
		return int32(val.(int8))
	case uint8:
		return int32(val.(uint8))
	case int16:
		return int32(val.(int16))
	case uint16:
		return int32(val.(uint16))
	case uint32:
		return int32(val.(uint32))
	case int64:
		return int32(val.(int64))
	case uint64:
		return int32(val.(uint64))
	case time.Time:
		return val.(time.Time).Format(time.RFC3339Nano)
	case []byte:
		return types.FormatBinary(val.([]byte))
	}
	return fmt.Sprintf("%v", val)
}

func ContainsAttribute(event cloudevents.Event, attributeName string) bool {
	if attributeName == "specversion" || attributeName == "id" || attributeName == "source" || attributeName == "type" {
		return true
	}

	if attr := spec.V1.Attribute(attributeName); attr != nil {
		return attr.Get(event.Context) != nil
	}

	_, ok := event.Extensions()[attributeName]
	return ok
}
