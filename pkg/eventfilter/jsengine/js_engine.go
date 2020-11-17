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
	"fmt"
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
	fakeProgram, err := parser.ParseFile(nil, "", src, 0)
	if err != nil {
		return nil, errors.WithStack(fmt.Errorf("error while parsing filter expression '%s': %w", src, err))
	}

	exprStmt, ok := fakeProgram.Body[0].(*ast.ExpressionStatement)
	if !ok {
		return nil, errors.WithStack(errors.New("program body should be just an expression: " + src))
	} else if err := staticAstCheck(src, exprStmt.Expression); err != nil {
		return nil, err
	}

	if len(fakeProgram.DeclarationList) != 0 {
		return nil, errors.WithStack(errors.New("found list of declared values, program body should be just an expression: " + src))
	}

	// Create the real ast with the function declaration the evaluator expects
	program, err := parser.ParseFile(nil, "", "function test(event) { return "+src+"; }", 0)
	if err != nil {
		return nil, errors.WithStack(errors.Wrap(err, "error while parsing the final script"))
	}

	// Generated ast should look like this:
	//&ast.Program{
	//	// Noop statement
	//	Body: []ast.Statement{&ast.EmptyStatement{}},
	//	DeclarationList: []ast.Declaration{&ast.FunctionDeclaration{
	//		Function: &ast.FunctionLiteral{
	//			Name:            &ast.Identifier{Name: unistring.NewFromString("test")},
	//			ParameterList:   &ast.ParameterList{List: []*ast.Identifier{{Name: unistring.NewFromString("event")}}},
	//			Body:            &ast.BlockStatement{
	//				List: []ast.Statement{
	//					&ast.ReturnStatement{Argument: exprStmt.Expression},
	//				},
	//			},
	//			Source:          "function test(event) { return " + src + "; }",
	//			DeclarationList: nil,
	//		},
	//	}},
	//	File: fakeProgram.File,
	//}

	return goja.CompileAST(program, false)
}

func staticAstCheck(originalSrc string, expressions ...ast.Expression) error {
	for _, expression := range expressions {
		switch e := expression.(type) {
		case *ast.ArrayLiteral:
			return staticAstCheck(originalSrc, e.Value...)
		case *ast.AssignExpression:
			return errors.WithStack(errors.New("found assignment statement, program body should be just an expression: " + originalSrc))
		case *ast.BinaryExpression:
			return staticAstCheck(originalSrc, e.Left, e.Right)
		case *ast.BracketExpression:
			return staticAstCheck(originalSrc, e.Left, e.Member)
		case *ast.CallExpression:
			return staticAstCheck(originalSrc, append(e.ArgumentList, e.Callee)...)
		case *ast.ConditionalExpression:
			return staticAstCheck(originalSrc, e.Test, e.Alternate, e.Consequent)
		case *ast.DotExpression:
			return staticAstCheck(originalSrc, e.Left)
		case *ast.FunctionLiteral:
			return errors.WithStack(errors.New("found function expression, program body should be just an expression: " + originalSrc))
		case *ast.NewExpression:
			return errors.WithStack(errors.New("found object instantiation expression, program body should be just an expression: " + originalSrc))
		case *ast.SequenceExpression:
			return staticAstCheck(originalSrc, e.Sequence...)
		case *ast.UnaryExpression:
			return staticAstCheck(originalSrc, e.Operand)
		case *ast.VariableExpression:
			return staticAstCheck(originalSrc, e.Initializer)
		}
	}
	return nil
}

func runFilter(event cloudevents.Event, vm *goja.Runtime) (bool, error) {
	eventObj, err := configureEventObject(vm, event)
	if err != nil {
		return false, err
	}
	testFn, ok := goja.AssertFunction(vm.Get("test"))
	if !ok {
		return false, errors.New("Something weird is going on here")
	}

	val, err := testFn(goja.Undefined(), eventObj)
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

func runProgramWithSafeTimeout(duration time.Duration, vm *goja.Runtime, program *goja.Program) (goja.Value, error) {
	t := time.AfterFunc(duration, func() {
		vm.Interrupt("filter instantiation timeout, stop running Doom on this filter!")
	})
	defer t.Stop()

	return vm.RunProgram(program)
}
