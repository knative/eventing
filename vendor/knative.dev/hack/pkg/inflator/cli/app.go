package cli

import (
	"fmt"

	"knative.dev/hack/pkg/inflator/extract"
	"knative.dev/hack/pkg/retcode"
)

// Execute will execute the application.
func Execute(opts []Option) Result {
	ex := Execution{}.Default().Configure(opts)
	fl, err := parseArgs(&ex)
	if err != nil {
		return Result{
			Execution: ex,
			Err:       err,
		}
	}
	op := createOperation(fl, ex.Args)
	return Result{
		Execution: ex,
		Err:       op.Extract(ex),
	}
}

// ExecuteOrDie will execute the application or perform os.Exit in case of error.
func ExecuteOrDie(opts ...Option) {
	if r := Execute(opts); r.Err != nil {
		r.PrintErrln(fmt.Sprintf("%v", r.Err))
		r.Exit(retcode.Calc(r.Err))
	}
}

func createOperation(fl *flags, argv []string) extract.Operation {
	return extract.Operation{
		ScriptName: argv[0],
		Verbose:    fl.verbose,
	}
}
