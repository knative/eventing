/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package runtime

import (
	"errors"
	"strings"

	cesql "github.com/cloudevents/sdk-go/sql/v2"
	"github.com/cloudevents/sdk-go/sql/v2/function"
)

type functionItem struct {
	fixedArgsFunctions map[int]cesql.Function
	variadicFunction   cesql.Function
}

type functionTable map[string]*functionItem

func (table functionTable) AddFunction(function cesql.Function) error {
	item := table[function.Name()]
	if item == nil {
		item = &functionItem{
			fixedArgsFunctions: make(map[int]cesql.Function),
		}
		table[function.Name()] = item
	}

	if function.IsVariadic() {
		if item.variadicFunction != nil {
			return errors.New("cannot add the variadic function, " +
				"because there is already another variadic function defined with the same name")
		}
		maxArity := -1
		for a := range item.fixedArgsFunctions {
			if a > maxArity {
				maxArity = a
			}
		}
		if maxArity >= function.Arity() {
			return errors.New("cannot add the variadic function, " +
				"because there is already another function defined with the same name and same or greater arity")
		}

		item.variadicFunction = function
		return nil
	} else {
		if _, ok := item.fixedArgsFunctions[function.Arity()]; ok {
			return errors.New("cannot add the function, " +
				"because there is already another function defined with the same arity and same name")
		}

		item.fixedArgsFunctions[function.Arity()] = function
		return nil
	}
}

func (table functionTable) ResolveFunction(name string, args int) cesql.Function {
	item := table[strings.ToUpper(name)]
	if item == nil {
		return nil
	}

	if fn, ok := item.fixedArgsFunctions[args]; ok {
		return fn
	}

	if item.variadicFunction == nil || item.variadicFunction.Arity() > args {
		return nil
	}

	return item.variadicFunction
}

var globalFunctionTable = functionTable{}

func init() {
	for _, fn := range []cesql.Function{
		function.IntFunction,
		function.BoolFunction,
		function.StringFunction,
		function.IsBoolFunction,
		function.IsIntFunction,
		function.AbsFunction,
		function.LengthFunction,
		function.ConcatFunction,
		function.ConcatWSFunction,
		function.LowerFunction,
		function.UpperFunction,
		function.TrimFunction,
		function.LeftFunction,
		function.RightFunction,
		function.SubstringFunction,
		function.SubstringWithLengthFunction,
	} {
		if err := globalFunctionTable.AddFunction(fn); err != nil {
			panic(err)
		}
	}
}

func ResolveFunction(name string, args int) cesql.Function {
	return globalFunctionTable.ResolveFunction(name, args)
}
