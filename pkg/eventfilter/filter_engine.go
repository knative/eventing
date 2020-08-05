package eventfilter

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

func RunFilter(event cloudevents.Event, program *goja.Program) (bool, error) {
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
		vm.Interrupt("execution timeout")
	})

	return vm.RunProgram(program)
}
