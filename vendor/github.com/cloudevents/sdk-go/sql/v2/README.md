# CloudEvents Expression Language Go implementation

CloudEvents Expression Language implementation.

Note: this package is a work in progress, APIs might break in future releases.

## User guide

To start using it:

```go
import cesqlparser "github.com/cloudevents/sdk-go/sql/v2/parser"

// Parse the expression
expression, err := cesqlparser.Parse("subject = 'Hello world'")

// Res can be either int32, bool or string
res, err := expression.Evaluate(event)
```

Add a user defined function
```go
import (
    cesql "github.com/cloudevents/sdk-go/sql/v2"
    cefn "github.com/cloudevents/sdk-go/sql/v2/function"
    cesqlparser "github.com/cloudevents/sdk-go/sql/v2/parser"
    ceruntime "github.com/cloudevents/sdk-go/sql/v2/runtime"
    cloudevents "github.com/cloudevents/sdk-go/v2"
)

// Create a test event
event := cloudevents.NewEvent()
event.SetID("aaaa-bbbb-dddd")
event.SetSource("https://my-source")
event.SetType("dev.tekton.event")

// Create and add a new user defined function
var HasPrefixFunction cesql.Function = cefn.NewFunction(
    "HASPREFIX",
    []cesql.Type{cesql.StringType, cesql.StringType},
    nil,
    func(event cloudevents.Event, i []interface{}) (interface{}, error) {
        str := i[0].(string)
        prefix := i[1].(string)

        return strings.HasPrefix(str, prefix), nil
    },
)

err := ceruntime.AddFunction(HasPrefixFunction)

// parse the expression 
expression, err := cesqlparser.Parse("HASPREFIX(type, 'dev.tekton.event')")
	if err != nil {
		fmt.Println("parser err: ", err)
		os.Exit(1)
	}

// Evalute the expression with the test event
res, err := expression.Evaluate(event)

if res.(bool) {
    fmt.Println("Event type has the prefix")
} else {
    fmt.Println("Event type doesn't have the prefix")
}
```

## Development guide

To regenerate the parser, make sure you have [ANTLR4 installed](https://github.com/antlr/antlr4/blob/master/doc/getting-started.md) and then run:

```shell
antlr4 -v 4.10.1 -Dlanguage=Go -package gen -o gen -visitor -no-listener CESQLParser.g4
```

Then you need to run this sed command as a workaround until this ANTLR [issue](https://github.com/antlr/antlr4/issues/2433) is resolved. Without this, building for 32bit platforms will throw an int overflow error: 
```shell
sed -i 's/(1<</(int64(1)<</g' gen/cesqlparser_parser.go
```
