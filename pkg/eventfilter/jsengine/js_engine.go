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
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/types"
	"github.com/dop251/goja"
	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
	"github.com/pkg/errors"
)

const timeout = time.Second * 2

// ParseFilterExpr parses src as a javascript filter expression.
// Returns error if src syntax is invalid or if src root AST element is not an expression statement.
func ParseFilterExpr(src string) (*goja.Program, error) {
	program, err := parser.ParseFile(nil, "", src, 0)
	if err != nil {
		return nil, errors.WithStack(errors.Wrap(err, "error while parsing filter expression"))
	}

	if _, ok := program.Body[0].(*ast.ExpressionStatement); !ok {
		return nil, errors.WithStack(errors.New("program body should be just an expression: " + src))
	}

	return goja.CompileAST(program, false)
}

func runFilter(event cloudevents.Event, program *goja.Program) (bool, error) {
	vm := goja.New()
	obj, err := configureEventObject(vm, event)
	if err != nil {
		return false, err
	}
	vm.Set("event", obj)

	val, err := runWithSafeTimeout(timeout, vm, program)
	if err != nil {
		return false, err
	}
	return val.ToBoolean(), nil
}

func configureEventObject(vm *goja.Runtime, event cloudevents.Event) (*goja.Object, error) {
	obj := vm.NewObject()
	attributeGetters := map[string]func() interface{}{
		"specversion": func() interface{} {
			return event.SpecVersion()
		},
		"type": func() interface{} {
			return event.Type()
		},
		"source": func() interface{} {
			return event.Source()
		},
		"subject": func() interface{} {
			return event.Subject()
		},
		"id": func() interface{} {
			return event.ID()
		},
		"time": func() interface{} {
			return event.Time()
		},
		"dataschema": func() interface{} {
			return event.DataSchema()
		},
		"datacontenttype": func() interface{} {
			return event.DataContentType()
		},
	}
	for name, fn := range attributeGetters {
		if err := bindValueExtractor(vm, name, fn, obj); err != nil {
			return nil, errors.Wrap(err, "error while binding attribute getter "+name)
		}
	}
	for name, val := range event.Extensions() {
		if err := bindValue(vm, name, val, obj); err != nil {
			return nil, errors.Wrap(err, "error while binding extension getter "+name)
		}
	}
	return obj, nil
}

func bindValue(vm *goja.Runtime, propName string, eventVal interface{}, obj *goja.Object) error {
	return obj.DefineAccessorProperty(propName, vm.ToValue(func(fc goja.FunctionCall) goja.Value {
		return coerceToJsTypes(vm, eventVal)
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)
}

func bindValueExtractor(vm *goja.Runtime, propName string, extractFn func() interface{}, obj *goja.Object) error {
	return obj.DefineAccessorProperty(propName, vm.ToValue(func(fc goja.FunctionCall) goja.Value {
		return coerceToJsTypes(vm, extractFn())
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)
}

func coerceToJsTypes(vm *goja.Runtime, val interface{}) goja.Value {
	if types.IsZero(val) {
		return goja.Null()
	}
	if t, ok := val.(time.Time); ok {
		val, err := vm.New(vm.GlobalObject().Get("Date").ToObject(vm), vm.ToValue(t.UnixNano()/1e6))
		if err != nil {
			panic(err)
		}
		return val
	}
	// String is a safe type, this can never fail
	return vm.ToValue(val)
}

func runWithSafeTimeout(duration time.Duration, vm *goja.Runtime, program *goja.Program) (goja.Value, error) {
	time.AfterFunc(duration, func() {
		vm.Interrupt("filter execution timeout, stop running Doom on this filter!")
	})

	return vm.RunProgram(program)
}
