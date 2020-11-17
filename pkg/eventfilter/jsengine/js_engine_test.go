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
	exprs := map[string]bool{
		`/w3schools/i.test(xxx)`:                    true,
		`function test(event) { return event.id; }`: false,
		// eval at runtime is not available, but it passes the ast check
		`eval("for i < 999999999999999999999999999999999999999999999999999999999999999999; i++{}")`: true,
		`new Function('x', 'y', 'return x+y')(1, 1)`:                                                false,
		`(a == 2) && a = 2`: false,
		`a = 2`:             false,
	}
	for e, r := range exprs {
		_, err := ParseFilterExpr(e)
		if r {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
}

func TestEventKeys(t *testing.T) {
	event := test.FullEvent()

	vm := goja.New()
	obj, err := configureEventObject(vm, event)
	require.NoError(t, err)

	_, err = vm.RunString("function test(event) { return Object.keys(event); }")
	require.NoError(t, err)

	testFn, ok := goja.AssertFunction(vm.Get("test"))
	require.True(t, ok)
	val, err := testFn(goja.Undefined(), obj)
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

	_, err = vm.RunString("function test(event) { return event.someint; }")
	require.NoError(t, err)

	testFn, ok := goja.AssertFunction(vm.Get("test"))
	require.True(t, ok)
	val, err := testFn(goja.Undefined(), obj)
	require.NoError(t, err)

	s := val.Export()
	require.Equal(t, int64(10), s)
}

func TestDate(t *testing.T) {
	event := test.FullEvent()

	vm := goja.New()
	obj, err := configureEventObject(vm, event)
	require.NoError(t, err)

	_, err = vm.RunString("function test(event) { return event.time.getFullYear(); }")
	require.NoError(t, err)

	testFn, ok := goja.AssertFunction(vm.Get("test"))
	require.True(t, ok)
	val, err := testFn(goja.Undefined(), obj)
	require.NoError(t, err)

	s := val.Export()
	require.Equal(t, int64(2020), s)
}
