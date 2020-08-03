package filter

import (
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/pkg/errors"
	"github.com/robertkrimen/otto"
	"github.com/robertkrimen/otto/ast"
	"github.com/robertkrimen/otto/parser"
)

const timeout = time.Second * 2

var haltErr = errors.New("Script ran over the timeout")

func ParseFilterExpr(src string) (*ast.Program, error) {
	program, err := parser.ParseFile(nil, "", src, 0)
	if err != nil {
		return nil, err
	}

	if _, ok := program.Body[0].(*ast.ExpressionStatement); !ok {
		return nil, errors.WithStack(errors.New("program body should be just an expression: " + src))
	}

	return program, nil
}

func RunFilter(event cloudevents.Event, program *ast.Program) (bool, error) {
	vm := otto.New()
	obj, err := configureEventObject(vm, event)
	if err != nil {
		return false, err
	}
	err = vm.Set("event", obj)
	if err != nil {
		return false, err
	}

	val, err := runWithSafeTimeout(timeout, vm, program)
	if err != nil {
		return false, err
	}
	return val.ToBoolean()
}

func configureEventObject(vm *otto.Otto, event cloudevents.Event) (*otto.Object, error) {
	obj, err := vm.Object(`({})`)
	if err != nil {
		return nil, err
	}
	for name, fn := range map[string]func() string{
		"id":              event.ID,
		"specversion":     event.SpecVersion,
		"type":            event.Type,
		"source":          event.Source,
		"datacontenttype": event.DataContentType,
		"subject":         event.Subject,
	} {
		if err := bindStringValue(name, fn, obj); err != nil {
			return nil, errors.Wrap(err, "Error while binding event object")
		}
	}
	return obj, nil
}

func bindStringValue(funcName string, extractFn func() string, obj *otto.Object) error {
	return obj.Set(funcName, func(otto.FunctionCall) otto.Value {
		eventVal := extractFn()
		if eventVal == "" {
			return otto.NullValue()
		}
		// String is a safe type, this can never fail
		v, _ := otto.ToValue(eventVal)
		return v
	})
}

func runWithSafeTimeout(duration time.Duration, vm *otto.Otto, program *ast.Program) (res otto.Value, err error) {
	defer func() {
		if caught := recover(); caught != nil {
			err = fmt.Errorf("catched panic while running the script %v", caught)
		}
	}()

	vm.Interrupt = make(chan func(), 1) // The buffer prevents blocking

	go func() {
		time.Sleep(duration) // Stop after two seconds
		vm.Interrupt <- func() {
			panic(haltErr)
		}
	}()

	res, err = vm.Run(program)
	return
}
