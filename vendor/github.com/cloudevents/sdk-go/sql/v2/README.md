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

## Development guide

To regenerate the parser, make sure you have [ANTLR4 installed](https://github.com/antlr/antlr4/blob/master/doc/getting-started.md) and then run:

```shell
antlr4 -Dlanguage=Go -package gen -o gen -visitor -no-listener CESQLParser.g4
```
