package paho

type (
	// Logger interface allows implementations to provide to this package any
	// object that implements the methods defined in it.
	Logger interface {
		Println(v ...interface{})
		Printf(format string, v ...interface{})
	}

	// NOOPLogger implements the logger that does not perform any operation
	// by default. This allows us to efficiently discard the unwanted messages.
	NOOPLogger struct{}
)

// Println is the library provided NOOPLogger's
// implementation of the required interface function()
func (NOOPLogger) Println(v ...interface{}) {}

// Printf is the library provided NOOPLogger's
// implementation of the required interface function(){}
func (NOOPLogger) Printf(format string, v ...interface{}) {}
