package cli

import "fmt"

// Print is a convenience method to Print to the defined output, fallback to Stderr if not set.
func (e Execution) Print(i ...interface{}) {
	fmt.Fprint(e.Stdout, i...)
}

// Println is a convenience method to Println to the defined output, fallback to Stderr if not set.
func (e Execution) Println(i ...interface{}) {
	e.Print(fmt.Sprintln(i...))
}

// Printf is a convenience method to Printf to the defined output, fallback to Stderr if not set.
func (e Execution) Printf(format string, i ...interface{}) {
	e.Print(fmt.Sprintf(format, i...))
}

// PrintErr is a convenience method to Print to the defined Err output, fallback to Stderr if not set.
func (e Execution) PrintErr(i ...interface{}) {
	fmt.Fprint(e.Stderr, i...)
}

// PrintErrln is a convenience method to Println to the defined Err output, fallback to Stderr if not set.
func (e Execution) PrintErrln(i ...interface{}) {
	e.PrintErr(fmt.Sprintln(i...))
}

// PrintErrf is a convenience method to Printf to the defined Err output, fallback to Stderr if not set.
func (e Execution) PrintErrf(format string, i ...interface{}) {
	e.PrintErr(fmt.Sprintf(format, i...))
}
