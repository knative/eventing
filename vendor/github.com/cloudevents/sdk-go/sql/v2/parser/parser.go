/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package parser

import (
	"fmt"

	"github.com/antlr/antlr4/runtime/Go/antlr"

	v2 "github.com/cloudevents/sdk-go/sql/v2"
	sqlerrors "github.com/cloudevents/sdk-go/sql/v2/errors"
	"github.com/cloudevents/sdk-go/sql/v2/gen"
)

type Parser struct {
	// TODO parser options
}

func (p *Parser) Parse(input string) (v2.Expression, error) {
	var is antlr.CharStream = antlr.NewInputStream(input)
	is = NewCaseChangingStream(is, true)

	// Create the JSON Lexer
	lexer := gen.NewCESQLParserLexer(is)
	var stream antlr.TokenStream = antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)

	// Create the JSON Parser
	antlrParser := gen.NewCESQLParserParser(stream)
	antlrParser.RemoveErrorListeners()
	collectingErrorListener := errorListener{}
	antlrParser.AddErrorListener(&collectingErrorListener)

	// Finally walk the tree
	visitor := expressionVisitor{}
	result := antlrParser.Cesql().Accept(&visitor)

	if result == nil {
		return nil, sqlerrors.NewParseError(append(collectingErrorListener.errs, visitor.parsingErrors...))
	}

	return result.(v2.Expression), sqlerrors.NewParseError(append(collectingErrorListener.errs, visitor.parsingErrors...))
}

type errorListener struct {
	antlr.DefaultErrorListener
	errs []error
}

func (d *errorListener) SyntaxError(recognizer antlr.Recognizer, offendingSymbol interface{}, line, column int, msg string, e antlr.RecognitionException) {
	d.errs = append(d.errs, fmt.Errorf("syntax error: %v", e.GetMessage()))
}

var defaultParser = Parser{}

func Parse(input string) (v2.Expression, error) {
	return defaultParser.Parse(input)
}
