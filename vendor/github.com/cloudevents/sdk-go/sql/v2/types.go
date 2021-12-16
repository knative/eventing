/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package v2

type Type uint8

const (
	StringType Type = iota
	IntegerType
	BooleanType
	AnyType
)

func TypePtr(t Type) *Type {
	return &t
}

func (t Type) IsSameType(val interface{}) bool {
	return TypeFromVal(val) == t
}

func (t Type) String() string {
	switch t {
	case IntegerType:
		return "Integer"
	case BooleanType:
		return "Boolean"
	case StringType:
		return "String"
	}
	return "Any"
}

func TypeFromVal(val interface{}) Type {
	switch val.(type) {
	case string:
		return StringType
	case int32:
		return IntegerType
	case bool:
		return BooleanType
	}
	return AnyType
}
