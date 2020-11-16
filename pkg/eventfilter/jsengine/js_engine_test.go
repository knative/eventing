/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package jsengine

import (
	"testing"

	"github.com/cloudevents/sdk-go/v2/test"
	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
)

func TestParseFilterExpr(t *testing.T) {
	program, err := ParseFilterExpr("/w3schools/i.test(xxx)")
	require.NoError(t, err)

	vm := goja.New()
	vm.Set("xxx", "w3schools")
	val, err := vm.RunProgram(program)
	require.NoError(t, err)

	b := val.ToBoolean()
	require.True(t, b)
}

func TestParseFilterExprHacky(t *testing.T) {
	program, err := ParseFilterExpr(`(function foo() { global.aaa = ""; console.log(aaa); })()`)
	require.NoError(t, err)
	require.NotNil(t, program)
}

//func TestAAA(t *testing.T) {
//	program, err := parser.ParseFile(nil, "", "function test(event) { return event.id; }", 0)
//
//	require.NoError(t, err)
//
//	vm := goja.New()
//	compiled, err := goja.CompileAST(program, false)
//	require.NoError(t, err)
//
//	vm.RunProgram(compiled)
//}
//
//func TestTimeout(t *testing.T) {
//	program, err := parser.ParseFile(nil, "", "while(true) {}", 0)
//	require.NoError(t, err)
//
//	compiled, err := goja.CompileAST(program, false)
//	require.NoError(t, err)
//
//	_, err = runFilter(test.MinEvent(), compiled)
//	require.IsType(t, &goja.InterruptedError{}, err, "error is an interrupted error")
//}

func TestParseFilterFailure(t *testing.T) {
	program, err := ParseFilterExpr(`function helloWorld() {}`)
	require.EqualError(t, err, "program body should be just an expression: function helloWorld() {}")
	require.Nil(t, program)
}

func TestEventKeys(t *testing.T) {
	event := test.FullEvent()

	program, err := ParseFilterExpr("Object.keys(event)")
	require.NoError(t, err)

	vm := goja.New()
	obj, err := configureEventObject(vm, event)
	require.NoError(t, err)

	vm.Set("event", obj)
	val, err := vm.RunProgram(program)
	require.NoError(t, err)

	s := val.Export()
	require.Contains(t, s, "specversion")
	require.Contains(t, s, "id")
	require.Contains(t, s, "type")
	require.Contains(t, s, "source")
	require.Contains(t, s, "subject")
	require.Contains(t, s, "time")
	require.Contains(t, s, "dataschema")
	require.Contains(t, s, "datacontenttype")

	require.Contains(t, s, "exbool")
	require.Contains(t, s, "exint")
	require.Contains(t, s, "exstring")
	require.Contains(t, s, "exbinary")
	require.Contains(t, s, "exurl")
	require.Contains(t, s, "extime")
}

func TestAccessExtension(t *testing.T) {
	event := test.FullEvent()
	event.SetExtension("someint", 10)

	vm := goja.New()
	obj, err := configureEventObject(vm, event)
	require.NoError(t, err)

	vm.Set("event", obj)
	val, err := vm.RunString("event.someint")
	require.NoError(t, err)

	s := val.Export()
	require.Equal(t, int64(10), s)
}

func TestDate(t *testing.T) {
	event := test.FullEvent()

	vm := goja.New()
	obj, err := configureEventObject(vm, event)
	require.NoError(t, err)

	vm.Set("event", obj)
	val, err := vm.RunString("event.time.getFullYear()")
	require.NoError(t, err)

	s := val.Export()
	require.Equal(t, int64(2020), s)
}

//func TestRunFilter(t *testing.T) {
//	event := test.FullEvent()
//
//	tests := []struct {
//		expression string
//		result     bool
//	}{{
//		expression: "event.id === \"" + event.ID() + "\"",
//		result:     true,
//	}, {
//		expression: "event.id !== \"" + event.ID() + "\"",
//		result:     false,
//	}, {
//		expression: `event.datacontenttype.indexOf("json") != -1 && true`,
//		result:     true,
//	}, {
//		expression: `event.time.getFullYear() == 2020`,
//		result:     true,
//	}, {
//		expression: `event.exint === 42`,
//		result:     true,
//	}, {
//		expression: fmt.Sprintf(`(event.type === "---%s") || (event.type === "%s" ? event.id !== "%s" : event.id === "%s")`, event.Type(), event.Type(), event.ID(), event.ID()),
//		result:     false,
//	}}
//	for _, tc := range tests {
//		t.Run(tc.expression, func(t *testing.T) {
//			program, err := ParseFilterExpr(tc.expression)
//			require.NoError(t, err)
//			res, err := runFilter(event, program)
//			require.NoError(t, err)
//			require.Equal(t, tc.result, res)
//		})
//	}
//}
