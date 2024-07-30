package cli

import (
	"io"
	"os"
)

// Execution is used to execute a command.
type Execution struct {
	Args   []string
	Stdout io.Writer
	Stderr io.Writer
	Exit   func(code int)
}

// Default will set default values for the execution.
func (e Execution) Default() Execution {
	if e.Stdout == nil {
		e.Stdout = os.Stdout
	}
	if e.Stderr == nil {
		e.Stderr = os.Stderr
	}
	if e.Exit == nil {
		e.Exit = os.Exit
	}
	if len(e.Args) == 0 {
		e.Args = os.Args[1:]
	}
	return e
}

// Configure will configure the execution.
func (e Execution) Configure(opts []Option) Execution {
	for _, opt := range opts {
		opt(&e)
	}
	return e
}

// Option is used to configure an App.
type Option func(*Execution)

// Options to override the commandline for testing purposes.
var Options []Option //nolint:gochecknoglobals

// Result is a result of execution.
type Result struct {
	Execution
	Err error
}
