/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

// Code generated from CESQLParser.g4 by ANTLR 4.9. DO NOT EDIT.

package gen // CESQLParser
import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/antlr/antlr4/runtime/Go/antlr"
)

// Suppress unused import errors
var _ = fmt.Printf
var _ = reflect.Copy
var _ = strconv.Itoa

var parserATN = []uint16{
	3, 24715, 42794, 33075, 47597, 16764, 15335, 30598, 22884, 3, 35, 112,
	4, 2, 9, 2, 4, 3, 9, 3, 4, 4, 9, 4, 4, 5, 9, 5, 4, 6, 9, 6, 4, 7, 9, 7,
	4, 8, 9, 8, 4, 9, 9, 9, 4, 10, 9, 10, 4, 11, 9, 11, 3, 2, 3, 2, 3, 2, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 5, 3, 41, 10, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 5, 3, 57, 10, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 5, 3, 63, 10, 3, 3, 3, 3, 3, 7, 3, 67, 10, 3, 12, 3, 14,
	3, 70, 11, 3, 3, 4, 3, 4, 3, 4, 3, 4, 5, 4, 76, 10, 4, 3, 5, 3, 5, 3, 6,
	3, 6, 3, 7, 3, 7, 3, 8, 3, 8, 3, 9, 3, 9, 3, 10, 3, 10, 3, 10, 3, 10, 7,
	10, 92, 10, 10, 12, 10, 14, 10, 95, 11, 10, 5, 10, 97, 10, 10, 3, 10, 3,
	10, 3, 11, 3, 11, 3, 11, 3, 11, 7, 11, 105, 10, 11, 12, 11, 14, 11, 108,
	11, 11, 3, 11, 3, 11, 3, 11, 2, 3, 4, 12, 2, 4, 6, 8, 10, 12, 14, 16, 18,
	20, 2, 10, 3, 2, 13, 15, 3, 2, 16, 17, 3, 2, 18, 24, 3, 2, 9, 11, 3, 2,
	33, 34, 4, 2, 33, 33, 35, 35, 3, 2, 28, 29, 3, 2, 30, 31, 2, 120, 2, 22,
	3, 2, 2, 2, 4, 40, 3, 2, 2, 2, 6, 75, 3, 2, 2, 2, 8, 77, 3, 2, 2, 2, 10,
	79, 3, 2, 2, 2, 12, 81, 3, 2, 2, 2, 14, 83, 3, 2, 2, 2, 16, 85, 3, 2, 2,
	2, 18, 87, 3, 2, 2, 2, 20, 100, 3, 2, 2, 2, 22, 23, 5, 4, 3, 2, 23, 24,
	7, 2, 2, 3, 24, 3, 3, 2, 2, 2, 25, 26, 8, 3, 1, 2, 26, 27, 5, 10, 6, 2,
	27, 28, 5, 18, 10, 2, 28, 41, 3, 2, 2, 2, 29, 30, 7, 12, 2, 2, 30, 41,
	5, 4, 3, 13, 31, 32, 7, 17, 2, 2, 32, 41, 5, 4, 3, 12, 33, 34, 7, 26, 2,
	2, 34, 41, 5, 8, 5, 2, 35, 36, 7, 4, 2, 2, 36, 37, 5, 4, 3, 2, 37, 38,
	7, 5, 2, 2, 38, 41, 3, 2, 2, 2, 39, 41, 5, 6, 4, 2, 40, 25, 3, 2, 2, 2,
	40, 29, 3, 2, 2, 2, 40, 31, 3, 2, 2, 2, 40, 33, 3, 2, 2, 2, 40, 35, 3,
	2, 2, 2, 40, 39, 3, 2, 2, 2, 41, 68, 3, 2, 2, 2, 42, 43, 12, 8, 2, 2, 43,
	44, 9, 2, 2, 2, 44, 67, 5, 4, 3, 9, 45, 46, 12, 7, 2, 2, 46, 47, 9, 3,
	2, 2, 47, 67, 5, 4, 3, 8, 48, 49, 12, 6, 2, 2, 49, 50, 9, 4, 2, 2, 50,
	67, 5, 4, 3, 7, 51, 52, 12, 5, 2, 2, 52, 53, 9, 5, 2, 2, 53, 67, 5, 4,
	3, 5, 54, 56, 12, 11, 2, 2, 55, 57, 7, 12, 2, 2, 56, 55, 3, 2, 2, 2, 56,
	57, 3, 2, 2, 2, 57, 58, 3, 2, 2, 2, 58, 59, 7, 25, 2, 2, 59, 67, 5, 14,
	8, 2, 60, 62, 12, 9, 2, 2, 61, 63, 7, 12, 2, 2, 62, 61, 3, 2, 2, 2, 62,
	63, 3, 2, 2, 2, 63, 64, 3, 2, 2, 2, 64, 65, 7, 27, 2, 2, 65, 67, 5, 20,
	11, 2, 66, 42, 3, 2, 2, 2, 66, 45, 3, 2, 2, 2, 66, 48, 3, 2, 2, 2, 66,
	51, 3, 2, 2, 2, 66, 54, 3, 2, 2, 2, 66, 60, 3, 2, 2, 2, 67, 70, 3, 2, 2,
	2, 68, 66, 3, 2, 2, 2, 68, 69, 3, 2, 2, 2, 69, 5, 3, 2, 2, 2, 70, 68, 3,
	2, 2, 2, 71, 76, 5, 12, 7, 2, 72, 76, 5, 16, 9, 2, 73, 76, 5, 14, 8, 2,
	74, 76, 5, 8, 5, 2, 75, 71, 3, 2, 2, 2, 75, 72, 3, 2, 2, 2, 75, 73, 3,
	2, 2, 2, 75, 74, 3, 2, 2, 2, 76, 7, 3, 2, 2, 2, 77, 78, 9, 6, 2, 2, 78,
	9, 3, 2, 2, 2, 79, 80, 9, 7, 2, 2, 80, 11, 3, 2, 2, 2, 81, 82, 9, 8, 2,
	2, 82, 13, 3, 2, 2, 2, 83, 84, 9, 9, 2, 2, 84, 15, 3, 2, 2, 2, 85, 86,
	7, 32, 2, 2, 86, 17, 3, 2, 2, 2, 87, 96, 7, 4, 2, 2, 88, 93, 5, 4, 3, 2,
	89, 90, 7, 6, 2, 2, 90, 92, 5, 4, 3, 2, 91, 89, 3, 2, 2, 2, 92, 95, 3,
	2, 2, 2, 93, 91, 3, 2, 2, 2, 93, 94, 3, 2, 2, 2, 94, 97, 3, 2, 2, 2, 95,
	93, 3, 2, 2, 2, 96, 88, 3, 2, 2, 2, 96, 97, 3, 2, 2, 2, 97, 98, 3, 2, 2,
	2, 98, 99, 7, 5, 2, 2, 99, 19, 3, 2, 2, 2, 100, 101, 7, 4, 2, 2, 101, 106,
	5, 4, 3, 2, 102, 103, 7, 6, 2, 2, 103, 105, 5, 4, 3, 2, 104, 102, 3, 2,
	2, 2, 105, 108, 3, 2, 2, 2, 106, 104, 3, 2, 2, 2, 106, 107, 3, 2, 2, 2,
	107, 109, 3, 2, 2, 2, 108, 106, 3, 2, 2, 2, 109, 110, 7, 5, 2, 2, 110,
	21, 3, 2, 2, 2, 11, 40, 56, 62, 66, 68, 75, 93, 96, 106,
}
var literalNames = []string{
	"", "", "'('", "')'", "','", "'''", "'\"'", "'AND'", "'OR'", "'XOR'", "'NOT'",
	"'*'", "'/'", "'%'", "'+'", "'-'", "'='", "'!='", "'>'", "'>='", "'<'",
	"'<>'", "'<='", "'LIKE'", "'EXISTS'", "'IN'", "'TRUE'", "'FALSE'",
}
var symbolicNames = []string{
	"", "SPACE", "LR_BRACKET", "RR_BRACKET", "COMMA", "SINGLE_QUOTE_SYMB",
	"DOUBLE_QUOTE_SYMB", "AND", "OR", "XOR", "NOT", "STAR", "DIVIDE", "MODULE",
	"PLUS", "MINUS", "EQUAL", "NOT_EQUAL", "GREATER", "GREATER_OR_EQUAL", "LESS",
	"LESS_GREATER", "LESS_OR_EQUAL", "LIKE", "EXISTS", "IN", "TRUE", "FALSE",
	"DQUOTED_STRING_LITERAL", "SQUOTED_STRING_LITERAL", "INTEGER_LITERAL",
	"IDENTIFIER", "IDENTIFIER_WITH_NUMBER", "FUNCTION_IDENTIFIER_WITH_UNDERSCORE",
}

var ruleNames = []string{
	"cesql", "expression", "atom", "identifier", "functionIdentifier", "booleanLiteral",
	"stringLiteral", "integerLiteral", "functionParameterList", "setExpression",
}

type CESQLParserParser struct {
	*antlr.BaseParser
}

// NewCESQLParserParser produces a new parser instance for the optional input antlr.TokenStream.
//
// The *CESQLParserParser instance produced may be reused by calling the SetInputStream method.
// The initial parser configuration is expensive to construct, and the object is not thread-safe;
// however, if used within a Golang sync.Pool, the construction cost amortizes well and the
// objects can be used in a thread-safe manner.
func NewCESQLParserParser(input antlr.TokenStream) *CESQLParserParser {
	this := new(CESQLParserParser)
	deserializer := antlr.NewATNDeserializer(nil)
	deserializedATN := deserializer.DeserializeFromUInt16(parserATN)
	decisionToDFA := make([]*antlr.DFA, len(deserializedATN.DecisionToState))
	for index, ds := range deserializedATN.DecisionToState {
		decisionToDFA[index] = antlr.NewDFA(ds, index)
	}
	this.BaseParser = antlr.NewBaseParser(input)

	this.Interpreter = antlr.NewParserATNSimulator(this, deserializedATN, decisionToDFA, antlr.NewPredictionContextCache())
	this.RuleNames = ruleNames
	this.LiteralNames = literalNames
	this.SymbolicNames = symbolicNames
	this.GrammarFileName = "CESQLParser.g4"

	return this
}

// CESQLParserParser tokens.
const (
	CESQLParserParserEOF                                 = antlr.TokenEOF
	CESQLParserParserSPACE                               = 1
	CESQLParserParserLR_BRACKET                          = 2
	CESQLParserParserRR_BRACKET                          = 3
	CESQLParserParserCOMMA                               = 4
	CESQLParserParserSINGLE_QUOTE_SYMB                   = 5
	CESQLParserParserDOUBLE_QUOTE_SYMB                   = 6
	CESQLParserParserAND                                 = 7
	CESQLParserParserOR                                  = 8
	CESQLParserParserXOR                                 = 9
	CESQLParserParserNOT                                 = 10
	CESQLParserParserSTAR                                = 11
	CESQLParserParserDIVIDE                              = 12
	CESQLParserParserMODULE                              = 13
	CESQLParserParserPLUS                                = 14
	CESQLParserParserMINUS                               = 15
	CESQLParserParserEQUAL                               = 16
	CESQLParserParserNOT_EQUAL                           = 17
	CESQLParserParserGREATER                             = 18
	CESQLParserParserGREATER_OR_EQUAL                    = 19
	CESQLParserParserLESS                                = 20
	CESQLParserParserLESS_GREATER                        = 21
	CESQLParserParserLESS_OR_EQUAL                       = 22
	CESQLParserParserLIKE                                = 23
	CESQLParserParserEXISTS                              = 24
	CESQLParserParserIN                                  = 25
	CESQLParserParserTRUE                                = 26
	CESQLParserParserFALSE                               = 27
	CESQLParserParserDQUOTED_STRING_LITERAL              = 28
	CESQLParserParserSQUOTED_STRING_LITERAL              = 29
	CESQLParserParserINTEGER_LITERAL                     = 30
	CESQLParserParserIDENTIFIER                          = 31
	CESQLParserParserIDENTIFIER_WITH_NUMBER              = 32
	CESQLParserParserFUNCTION_IDENTIFIER_WITH_UNDERSCORE = 33
)

// CESQLParserParser rules.
const (
	CESQLParserParserRULE_cesql                 = 0
	CESQLParserParserRULE_expression            = 1
	CESQLParserParserRULE_atom                  = 2
	CESQLParserParserRULE_identifier            = 3
	CESQLParserParserRULE_functionIdentifier    = 4
	CESQLParserParserRULE_booleanLiteral        = 5
	CESQLParserParserRULE_stringLiteral         = 6
	CESQLParserParserRULE_integerLiteral        = 7
	CESQLParserParserRULE_functionParameterList = 8
	CESQLParserParserRULE_setExpression         = 9
)

// ICesqlContext is an interface to support dynamic dispatch.
type ICesqlContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsCesqlContext differentiates from other interfaces.
	IsCesqlContext()
}

type CesqlContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyCesqlContext() *CesqlContext {
	var p = new(CesqlContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = CESQLParserParserRULE_cesql
	return p
}

func (*CesqlContext) IsCesqlContext() {}

func NewCesqlContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *CesqlContext {
	var p = new(CesqlContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = CESQLParserParserRULE_cesql

	return p
}

func (s *CesqlContext) GetParser() antlr.Parser { return s.parser }

func (s *CesqlContext) Expression() IExpressionContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IExpressionContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IExpressionContext)
}

func (s *CesqlContext) EOF() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserEOF, 0)
}

func (s *CesqlContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *CesqlContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *CesqlContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitCesql(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CESQLParserParser) Cesql() (localctx ICesqlContext) {
	localctx = NewCesqlContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 0, CESQLParserParserRULE_cesql)

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(20)
		p.expression(0)
	}
	{
		p.SetState(21)
		p.Match(CESQLParserParserEOF)
	}

	return localctx
}

// IExpressionContext is an interface to support dynamic dispatch.
type IExpressionContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsExpressionContext differentiates from other interfaces.
	IsExpressionContext()
}

type ExpressionContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyExpressionContext() *ExpressionContext {
	var p = new(ExpressionContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = CESQLParserParserRULE_expression
	return p
}

func (*ExpressionContext) IsExpressionContext() {}

func NewExpressionContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ExpressionContext {
	var p = new(ExpressionContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = CESQLParserParserRULE_expression

	return p
}

func (s *ExpressionContext) GetParser() antlr.Parser { return s.parser }

func (s *ExpressionContext) CopyFrom(ctx *ExpressionContext) {
	s.BaseParserRuleContext.CopyFrom(ctx.BaseParserRuleContext)
}

func (s *ExpressionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ExpressionContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

type InExpressionContext struct {
	*ExpressionContext
}

func NewInExpressionContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *InExpressionContext {
	var p = new(InExpressionContext)

	p.ExpressionContext = NewEmptyExpressionContext()
	p.parser = parser
	p.CopyFrom(ctx.(*ExpressionContext))

	return p
}

func (s *InExpressionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *InExpressionContext) Expression() IExpressionContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IExpressionContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IExpressionContext)
}

func (s *InExpressionContext) IN() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserIN, 0)
}

func (s *InExpressionContext) SetExpression() ISetExpressionContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*ISetExpressionContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(ISetExpressionContext)
}

func (s *InExpressionContext) NOT() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserNOT, 0)
}

func (s *InExpressionContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitInExpression(s)

	default:
		return t.VisitChildren(s)
	}
}

type BinaryComparisonExpressionContext struct {
	*ExpressionContext
}

func NewBinaryComparisonExpressionContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *BinaryComparisonExpressionContext {
	var p = new(BinaryComparisonExpressionContext)

	p.ExpressionContext = NewEmptyExpressionContext()
	p.parser = parser
	p.CopyFrom(ctx.(*ExpressionContext))

	return p
}

func (s *BinaryComparisonExpressionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *BinaryComparisonExpressionContext) AllExpression() []IExpressionContext {
	var ts = s.GetTypedRuleContexts(reflect.TypeOf((*IExpressionContext)(nil)).Elem())
	var tst = make([]IExpressionContext, len(ts))

	for i, t := range ts {
		if t != nil {
			tst[i] = t.(IExpressionContext)
		}
	}

	return tst
}

func (s *BinaryComparisonExpressionContext) Expression(i int) IExpressionContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IExpressionContext)(nil)).Elem(), i)

	if t == nil {
		return nil
	}

	return t.(IExpressionContext)
}

func (s *BinaryComparisonExpressionContext) EQUAL() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserEQUAL, 0)
}

func (s *BinaryComparisonExpressionContext) NOT_EQUAL() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserNOT_EQUAL, 0)
}

func (s *BinaryComparisonExpressionContext) LESS_GREATER() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserLESS_GREATER, 0)
}

func (s *BinaryComparisonExpressionContext) GREATER_OR_EQUAL() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserGREATER_OR_EQUAL, 0)
}

func (s *BinaryComparisonExpressionContext) LESS_OR_EQUAL() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserLESS_OR_EQUAL, 0)
}

func (s *BinaryComparisonExpressionContext) LESS() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserLESS, 0)
}

func (s *BinaryComparisonExpressionContext) GREATER() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserGREATER, 0)
}

func (s *BinaryComparisonExpressionContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitBinaryComparisonExpression(s)

	default:
		return t.VisitChildren(s)
	}
}

type AtomExpressionContext struct {
	*ExpressionContext
}

func NewAtomExpressionContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *AtomExpressionContext {
	var p = new(AtomExpressionContext)

	p.ExpressionContext = NewEmptyExpressionContext()
	p.parser = parser
	p.CopyFrom(ctx.(*ExpressionContext))

	return p
}

func (s *AtomExpressionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *AtomExpressionContext) Atom() IAtomContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IAtomContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IAtomContext)
}

func (s *AtomExpressionContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitAtomExpression(s)

	default:
		return t.VisitChildren(s)
	}
}

type ExistsExpressionContext struct {
	*ExpressionContext
}

func NewExistsExpressionContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *ExistsExpressionContext {
	var p = new(ExistsExpressionContext)

	p.ExpressionContext = NewEmptyExpressionContext()
	p.parser = parser
	p.CopyFrom(ctx.(*ExpressionContext))

	return p
}

func (s *ExistsExpressionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ExistsExpressionContext) EXISTS() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserEXISTS, 0)
}

func (s *ExistsExpressionContext) Identifier() IIdentifierContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IIdentifierContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IIdentifierContext)
}

func (s *ExistsExpressionContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitExistsExpression(s)

	default:
		return t.VisitChildren(s)
	}
}

type BinaryLogicExpressionContext struct {
	*ExpressionContext
}

func NewBinaryLogicExpressionContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *BinaryLogicExpressionContext {
	var p = new(BinaryLogicExpressionContext)

	p.ExpressionContext = NewEmptyExpressionContext()
	p.parser = parser
	p.CopyFrom(ctx.(*ExpressionContext))

	return p
}

func (s *BinaryLogicExpressionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *BinaryLogicExpressionContext) AllExpression() []IExpressionContext {
	var ts = s.GetTypedRuleContexts(reflect.TypeOf((*IExpressionContext)(nil)).Elem())
	var tst = make([]IExpressionContext, len(ts))

	for i, t := range ts {
		if t != nil {
			tst[i] = t.(IExpressionContext)
		}
	}

	return tst
}

func (s *BinaryLogicExpressionContext) Expression(i int) IExpressionContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IExpressionContext)(nil)).Elem(), i)

	if t == nil {
		return nil
	}

	return t.(IExpressionContext)
}

func (s *BinaryLogicExpressionContext) AND() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserAND, 0)
}

func (s *BinaryLogicExpressionContext) OR() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserOR, 0)
}

func (s *BinaryLogicExpressionContext) XOR() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserXOR, 0)
}

func (s *BinaryLogicExpressionContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitBinaryLogicExpression(s)

	default:
		return t.VisitChildren(s)
	}
}

type LikeExpressionContext struct {
	*ExpressionContext
}

func NewLikeExpressionContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *LikeExpressionContext {
	var p = new(LikeExpressionContext)

	p.ExpressionContext = NewEmptyExpressionContext()
	p.parser = parser
	p.CopyFrom(ctx.(*ExpressionContext))

	return p
}

func (s *LikeExpressionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *LikeExpressionContext) Expression() IExpressionContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IExpressionContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IExpressionContext)
}

func (s *LikeExpressionContext) LIKE() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserLIKE, 0)
}

func (s *LikeExpressionContext) StringLiteral() IStringLiteralContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IStringLiteralContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IStringLiteralContext)
}

func (s *LikeExpressionContext) NOT() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserNOT, 0)
}

func (s *LikeExpressionContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitLikeExpression(s)

	default:
		return t.VisitChildren(s)
	}
}

type FunctionInvocationExpressionContext struct {
	*ExpressionContext
}

func NewFunctionInvocationExpressionContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *FunctionInvocationExpressionContext {
	var p = new(FunctionInvocationExpressionContext)

	p.ExpressionContext = NewEmptyExpressionContext()
	p.parser = parser
	p.CopyFrom(ctx.(*ExpressionContext))

	return p
}

func (s *FunctionInvocationExpressionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *FunctionInvocationExpressionContext) FunctionIdentifier() IFunctionIdentifierContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IFunctionIdentifierContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IFunctionIdentifierContext)
}

func (s *FunctionInvocationExpressionContext) FunctionParameterList() IFunctionParameterListContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IFunctionParameterListContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IFunctionParameterListContext)
}

func (s *FunctionInvocationExpressionContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitFunctionInvocationExpression(s)

	default:
		return t.VisitChildren(s)
	}
}

type BinaryMultiplicativeExpressionContext struct {
	*ExpressionContext
}

func NewBinaryMultiplicativeExpressionContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *BinaryMultiplicativeExpressionContext {
	var p = new(BinaryMultiplicativeExpressionContext)

	p.ExpressionContext = NewEmptyExpressionContext()
	p.parser = parser
	p.CopyFrom(ctx.(*ExpressionContext))

	return p
}

func (s *BinaryMultiplicativeExpressionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *BinaryMultiplicativeExpressionContext) AllExpression() []IExpressionContext {
	var ts = s.GetTypedRuleContexts(reflect.TypeOf((*IExpressionContext)(nil)).Elem())
	var tst = make([]IExpressionContext, len(ts))

	for i, t := range ts {
		if t != nil {
			tst[i] = t.(IExpressionContext)
		}
	}

	return tst
}

func (s *BinaryMultiplicativeExpressionContext) Expression(i int) IExpressionContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IExpressionContext)(nil)).Elem(), i)

	if t == nil {
		return nil
	}

	return t.(IExpressionContext)
}

func (s *BinaryMultiplicativeExpressionContext) STAR() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserSTAR, 0)
}

func (s *BinaryMultiplicativeExpressionContext) DIVIDE() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserDIVIDE, 0)
}

func (s *BinaryMultiplicativeExpressionContext) MODULE() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserMODULE, 0)
}

func (s *BinaryMultiplicativeExpressionContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitBinaryMultiplicativeExpression(s)

	default:
		return t.VisitChildren(s)
	}
}

type UnaryLogicExpressionContext struct {
	*ExpressionContext
}

func NewUnaryLogicExpressionContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *UnaryLogicExpressionContext {
	var p = new(UnaryLogicExpressionContext)

	p.ExpressionContext = NewEmptyExpressionContext()
	p.parser = parser
	p.CopyFrom(ctx.(*ExpressionContext))

	return p
}

func (s *UnaryLogicExpressionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *UnaryLogicExpressionContext) NOT() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserNOT, 0)
}

func (s *UnaryLogicExpressionContext) Expression() IExpressionContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IExpressionContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IExpressionContext)
}

func (s *UnaryLogicExpressionContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitUnaryLogicExpression(s)

	default:
		return t.VisitChildren(s)
	}
}

type UnaryNumericExpressionContext struct {
	*ExpressionContext
}

func NewUnaryNumericExpressionContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *UnaryNumericExpressionContext {
	var p = new(UnaryNumericExpressionContext)

	p.ExpressionContext = NewEmptyExpressionContext()
	p.parser = parser
	p.CopyFrom(ctx.(*ExpressionContext))

	return p
}

func (s *UnaryNumericExpressionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *UnaryNumericExpressionContext) MINUS() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserMINUS, 0)
}

func (s *UnaryNumericExpressionContext) Expression() IExpressionContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IExpressionContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IExpressionContext)
}

func (s *UnaryNumericExpressionContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitUnaryNumericExpression(s)

	default:
		return t.VisitChildren(s)
	}
}

type SubExpressionContext struct {
	*ExpressionContext
}

func NewSubExpressionContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *SubExpressionContext {
	var p = new(SubExpressionContext)

	p.ExpressionContext = NewEmptyExpressionContext()
	p.parser = parser
	p.CopyFrom(ctx.(*ExpressionContext))

	return p
}

func (s *SubExpressionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *SubExpressionContext) LR_BRACKET() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserLR_BRACKET, 0)
}

func (s *SubExpressionContext) Expression() IExpressionContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IExpressionContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IExpressionContext)
}

func (s *SubExpressionContext) RR_BRACKET() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserRR_BRACKET, 0)
}

func (s *SubExpressionContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitSubExpression(s)

	default:
		return t.VisitChildren(s)
	}
}

type BinaryAdditiveExpressionContext struct {
	*ExpressionContext
}

func NewBinaryAdditiveExpressionContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *BinaryAdditiveExpressionContext {
	var p = new(BinaryAdditiveExpressionContext)

	p.ExpressionContext = NewEmptyExpressionContext()
	p.parser = parser
	p.CopyFrom(ctx.(*ExpressionContext))

	return p
}

func (s *BinaryAdditiveExpressionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *BinaryAdditiveExpressionContext) AllExpression() []IExpressionContext {
	var ts = s.GetTypedRuleContexts(reflect.TypeOf((*IExpressionContext)(nil)).Elem())
	var tst = make([]IExpressionContext, len(ts))

	for i, t := range ts {
		if t != nil {
			tst[i] = t.(IExpressionContext)
		}
	}

	return tst
}

func (s *BinaryAdditiveExpressionContext) Expression(i int) IExpressionContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IExpressionContext)(nil)).Elem(), i)

	if t == nil {
		return nil
	}

	return t.(IExpressionContext)
}

func (s *BinaryAdditiveExpressionContext) PLUS() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserPLUS, 0)
}

func (s *BinaryAdditiveExpressionContext) MINUS() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserMINUS, 0)
}

func (s *BinaryAdditiveExpressionContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitBinaryAdditiveExpression(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CESQLParserParser) Expression() (localctx IExpressionContext) {
	return p.expression(0)
}

func (p *CESQLParserParser) expression(_p int) (localctx IExpressionContext) {
	var _parentctx antlr.ParserRuleContext = p.GetParserRuleContext()
	_parentState := p.GetState()
	localctx = NewExpressionContext(p, p.GetParserRuleContext(), _parentState)
	var _prevctx IExpressionContext = localctx
	var _ antlr.ParserRuleContext = _prevctx // TODO: To prevent unused variable warning.
	_startState := 2
	p.EnterRecursionRule(localctx, 2, CESQLParserParserRULE_expression, _p)
	var _la int

	defer func() {
		p.UnrollRecursionContexts(_parentctx)
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	var _alt int

	p.EnterOuterAlt(localctx, 1)
	p.SetState(38)
	p.GetErrorHandler().Sync(p)
	switch p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 0, p.GetParserRuleContext()) {
	case 1:
		localctx = NewFunctionInvocationExpressionContext(p, localctx)
		p.SetParserRuleContext(localctx)
		_prevctx = localctx

		{
			p.SetState(24)
			p.FunctionIdentifier()
		}
		{
			p.SetState(25)
			p.FunctionParameterList()
		}

	case 2:
		localctx = NewUnaryLogicExpressionContext(p, localctx)
		p.SetParserRuleContext(localctx)
		_prevctx = localctx
		{
			p.SetState(27)
			p.Match(CESQLParserParserNOT)
		}
		{
			p.SetState(28)
			p.expression(11)
		}

	case 3:
		localctx = NewUnaryNumericExpressionContext(p, localctx)
		p.SetParserRuleContext(localctx)
		_prevctx = localctx
		{
			p.SetState(29)
			p.Match(CESQLParserParserMINUS)
		}
		{
			p.SetState(30)
			p.expression(10)
		}

	case 4:
		localctx = NewExistsExpressionContext(p, localctx)
		p.SetParserRuleContext(localctx)
		_prevctx = localctx
		{
			p.SetState(31)
			p.Match(CESQLParserParserEXISTS)
		}
		{
			p.SetState(32)
			p.Identifier()
		}

	case 5:
		localctx = NewSubExpressionContext(p, localctx)
		p.SetParserRuleContext(localctx)
		_prevctx = localctx
		{
			p.SetState(33)
			p.Match(CESQLParserParserLR_BRACKET)
		}
		{
			p.SetState(34)
			p.expression(0)
		}
		{
			p.SetState(35)
			p.Match(CESQLParserParserRR_BRACKET)
		}

	case 6:
		localctx = NewAtomExpressionContext(p, localctx)
		p.SetParserRuleContext(localctx)
		_prevctx = localctx
		{
			p.SetState(37)
			p.Atom()
		}

	}
	p.GetParserRuleContext().SetStop(p.GetTokenStream().LT(-1))
	p.SetState(66)
	p.GetErrorHandler().Sync(p)
	_alt = p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 4, p.GetParserRuleContext())

	for _alt != 2 && _alt != antlr.ATNInvalidAltNumber {
		if _alt == 1 {
			if p.GetParseListeners() != nil {
				p.TriggerExitRuleEvent()
			}
			_prevctx = localctx
			p.SetState(64)
			p.GetErrorHandler().Sync(p)
			switch p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 3, p.GetParserRuleContext()) {
			case 1:
				localctx = NewBinaryMultiplicativeExpressionContext(p, NewExpressionContext(p, _parentctx, _parentState))
				p.PushNewRecursionContext(localctx, _startState, CESQLParserParserRULE_expression)
				p.SetState(40)

				if !(p.Precpred(p.GetParserRuleContext(), 6)) {
					panic(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 6)", ""))
				}
				{
					p.SetState(41)
					_la = p.GetTokenStream().LA(1)

					if !(((_la)&-(0x1f+1)) == 0 && ((int64(1)<<uint(_la))&((int64(1)<<CESQLParserParserSTAR)|(int64(1)<<CESQLParserParserDIVIDE)|(int64(1)<<CESQLParserParserMODULE))) != 0) {
						p.GetErrorHandler().RecoverInline(p)
					} else {
						p.GetErrorHandler().ReportMatch(p)
						p.Consume()
					}
				}
				{
					p.SetState(42)
					p.expression(7)
				}

			case 2:
				localctx = NewBinaryAdditiveExpressionContext(p, NewExpressionContext(p, _parentctx, _parentState))
				p.PushNewRecursionContext(localctx, _startState, CESQLParserParserRULE_expression)
				p.SetState(43)

				if !(p.Precpred(p.GetParserRuleContext(), 5)) {
					panic(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 5)", ""))
				}
				{
					p.SetState(44)
					_la = p.GetTokenStream().LA(1)

					if !(_la == CESQLParserParserPLUS || _la == CESQLParserParserMINUS) {
						p.GetErrorHandler().RecoverInline(p)
					} else {
						p.GetErrorHandler().ReportMatch(p)
						p.Consume()
					}
				}
				{
					p.SetState(45)
					p.expression(6)
				}

			case 3:
				localctx = NewBinaryComparisonExpressionContext(p, NewExpressionContext(p, _parentctx, _parentState))
				p.PushNewRecursionContext(localctx, _startState, CESQLParserParserRULE_expression)
				p.SetState(46)

				if !(p.Precpred(p.GetParserRuleContext(), 4)) {
					panic(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 4)", ""))
				}
				{
					p.SetState(47)
					_la = p.GetTokenStream().LA(1)

					if !(((_la)&-(0x1f+1)) == 0 && ((int64(1)<<uint(_la))&((int64(1)<<CESQLParserParserEQUAL)|(int64(1)<<CESQLParserParserNOT_EQUAL)|(int64(1)<<CESQLParserParserGREATER)|(int64(1)<<CESQLParserParserGREATER_OR_EQUAL)|(int64(1)<<CESQLParserParserLESS)|(int64(1)<<CESQLParserParserLESS_GREATER)|(int64(1)<<CESQLParserParserLESS_OR_EQUAL))) != 0) {
						p.GetErrorHandler().RecoverInline(p)
					} else {
						p.GetErrorHandler().ReportMatch(p)
						p.Consume()
					}
				}
				{
					p.SetState(48)
					p.expression(5)
				}

			case 4:
				localctx = NewBinaryLogicExpressionContext(p, NewExpressionContext(p, _parentctx, _parentState))
				p.PushNewRecursionContext(localctx, _startState, CESQLParserParserRULE_expression)
				p.SetState(49)

				if !(p.Precpred(p.GetParserRuleContext(), 3)) {
					panic(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 3)", ""))
				}
				{
					p.SetState(50)
					_la = p.GetTokenStream().LA(1)

					if !(((_la)&-(0x1f+1)) == 0 && ((int64(1)<<uint(_la))&((int64(1)<<CESQLParserParserAND)|(int64(1)<<CESQLParserParserOR)|(int64(1)<<CESQLParserParserXOR))) != 0) {
						p.GetErrorHandler().RecoverInline(p)
					} else {
						p.GetErrorHandler().ReportMatch(p)
						p.Consume()
					}
				}
				{
					p.SetState(51)
					p.expression(3)
				}

			case 5:
				localctx = NewLikeExpressionContext(p, NewExpressionContext(p, _parentctx, _parentState))
				p.PushNewRecursionContext(localctx, _startState, CESQLParserParserRULE_expression)
				p.SetState(52)

				if !(p.Precpred(p.GetParserRuleContext(), 9)) {
					panic(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 9)", ""))
				}
				p.SetState(54)
				p.GetErrorHandler().Sync(p)
				_la = p.GetTokenStream().LA(1)

				if _la == CESQLParserParserNOT {
					{
						p.SetState(53)
						p.Match(CESQLParserParserNOT)
					}

				}
				{
					p.SetState(56)
					p.Match(CESQLParserParserLIKE)
				}
				{
					p.SetState(57)
					p.StringLiteral()
				}

			case 6:
				localctx = NewInExpressionContext(p, NewExpressionContext(p, _parentctx, _parentState))
				p.PushNewRecursionContext(localctx, _startState, CESQLParserParserRULE_expression)
				p.SetState(58)

				if !(p.Precpred(p.GetParserRuleContext(), 7)) {
					panic(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 7)", ""))
				}
				p.SetState(60)
				p.GetErrorHandler().Sync(p)
				_la = p.GetTokenStream().LA(1)

				if _la == CESQLParserParserNOT {
					{
						p.SetState(59)
						p.Match(CESQLParserParserNOT)
					}

				}
				{
					p.SetState(62)
					p.Match(CESQLParserParserIN)
				}
				{
					p.SetState(63)
					p.SetExpression()
				}

			}

		}
		p.SetState(68)
		p.GetErrorHandler().Sync(p)
		_alt = p.GetInterpreter().AdaptivePredict(p.GetTokenStream(), 4, p.GetParserRuleContext())
	}

	return localctx
}

// IAtomContext is an interface to support dynamic dispatch.
type IAtomContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsAtomContext differentiates from other interfaces.
	IsAtomContext()
}

type AtomContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyAtomContext() *AtomContext {
	var p = new(AtomContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = CESQLParserParserRULE_atom
	return p
}

func (*AtomContext) IsAtomContext() {}

func NewAtomContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *AtomContext {
	var p = new(AtomContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = CESQLParserParserRULE_atom

	return p
}

func (s *AtomContext) GetParser() antlr.Parser { return s.parser }

func (s *AtomContext) CopyFrom(ctx *AtomContext) {
	s.BaseParserRuleContext.CopyFrom(ctx.BaseParserRuleContext)
}

func (s *AtomContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *AtomContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

type BooleanAtomContext struct {
	*AtomContext
}

func NewBooleanAtomContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *BooleanAtomContext {
	var p = new(BooleanAtomContext)

	p.AtomContext = NewEmptyAtomContext()
	p.parser = parser
	p.CopyFrom(ctx.(*AtomContext))

	return p
}

func (s *BooleanAtomContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *BooleanAtomContext) BooleanLiteral() IBooleanLiteralContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IBooleanLiteralContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IBooleanLiteralContext)
}

func (s *BooleanAtomContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitBooleanAtom(s)

	default:
		return t.VisitChildren(s)
	}
}

type IdentifierAtomContext struct {
	*AtomContext
}

func NewIdentifierAtomContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *IdentifierAtomContext {
	var p = new(IdentifierAtomContext)

	p.AtomContext = NewEmptyAtomContext()
	p.parser = parser
	p.CopyFrom(ctx.(*AtomContext))

	return p
}

func (s *IdentifierAtomContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *IdentifierAtomContext) Identifier() IIdentifierContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IIdentifierContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IIdentifierContext)
}

func (s *IdentifierAtomContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitIdentifierAtom(s)

	default:
		return t.VisitChildren(s)
	}
}

type StringAtomContext struct {
	*AtomContext
}

func NewStringAtomContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *StringAtomContext {
	var p = new(StringAtomContext)

	p.AtomContext = NewEmptyAtomContext()
	p.parser = parser
	p.CopyFrom(ctx.(*AtomContext))

	return p
}

func (s *StringAtomContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *StringAtomContext) StringLiteral() IStringLiteralContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IStringLiteralContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IStringLiteralContext)
}

func (s *StringAtomContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitStringAtom(s)

	default:
		return t.VisitChildren(s)
	}
}

type IntegerAtomContext struct {
	*AtomContext
}

func NewIntegerAtomContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *IntegerAtomContext {
	var p = new(IntegerAtomContext)

	p.AtomContext = NewEmptyAtomContext()
	p.parser = parser
	p.CopyFrom(ctx.(*AtomContext))

	return p
}

func (s *IntegerAtomContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *IntegerAtomContext) IntegerLiteral() IIntegerLiteralContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IIntegerLiteralContext)(nil)).Elem(), 0)

	if t == nil {
		return nil
	}

	return t.(IIntegerLiteralContext)
}

func (s *IntegerAtomContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitIntegerAtom(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CESQLParserParser) Atom() (localctx IAtomContext) {
	localctx = NewAtomContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 4, CESQLParserParserRULE_atom)

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.SetState(73)
	p.GetErrorHandler().Sync(p)

	switch p.GetTokenStream().LA(1) {
	case CESQLParserParserTRUE, CESQLParserParserFALSE:
		localctx = NewBooleanAtomContext(p, localctx)
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(69)
			p.BooleanLiteral()
		}

	case CESQLParserParserINTEGER_LITERAL:
		localctx = NewIntegerAtomContext(p, localctx)
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(70)
			p.IntegerLiteral()
		}

	case CESQLParserParserDQUOTED_STRING_LITERAL, CESQLParserParserSQUOTED_STRING_LITERAL:
		localctx = NewStringAtomContext(p, localctx)
		p.EnterOuterAlt(localctx, 3)
		{
			p.SetState(71)
			p.StringLiteral()
		}

	case CESQLParserParserIDENTIFIER, CESQLParserParserIDENTIFIER_WITH_NUMBER:
		localctx = NewIdentifierAtomContext(p, localctx)
		p.EnterOuterAlt(localctx, 4)
		{
			p.SetState(72)
			p.Identifier()
		}

	default:
		panic(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
	}

	return localctx
}

// IIdentifierContext is an interface to support dynamic dispatch.
type IIdentifierContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsIdentifierContext differentiates from other interfaces.
	IsIdentifierContext()
}

type IdentifierContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyIdentifierContext() *IdentifierContext {
	var p = new(IdentifierContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = CESQLParserParserRULE_identifier
	return p
}

func (*IdentifierContext) IsIdentifierContext() {}

func NewIdentifierContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *IdentifierContext {
	var p = new(IdentifierContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = CESQLParserParserRULE_identifier

	return p
}

func (s *IdentifierContext) GetParser() antlr.Parser { return s.parser }

func (s *IdentifierContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserIDENTIFIER, 0)
}

func (s *IdentifierContext) IDENTIFIER_WITH_NUMBER() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserIDENTIFIER_WITH_NUMBER, 0)
}

func (s *IdentifierContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *IdentifierContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *IdentifierContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitIdentifier(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CESQLParserParser) Identifier() (localctx IIdentifierContext) {
	localctx = NewIdentifierContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 6, CESQLParserParserRULE_identifier)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(75)
		_la = p.GetTokenStream().LA(1)

		if !(_la == CESQLParserParserIDENTIFIER || _la == CESQLParserParserIDENTIFIER_WITH_NUMBER) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}

	return localctx
}

// IFunctionIdentifierContext is an interface to support dynamic dispatch.
type IFunctionIdentifierContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsFunctionIdentifierContext differentiates from other interfaces.
	IsFunctionIdentifierContext()
}

type FunctionIdentifierContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyFunctionIdentifierContext() *FunctionIdentifierContext {
	var p = new(FunctionIdentifierContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = CESQLParserParserRULE_functionIdentifier
	return p
}

func (*FunctionIdentifierContext) IsFunctionIdentifierContext() {}

func NewFunctionIdentifierContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *FunctionIdentifierContext {
	var p = new(FunctionIdentifierContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = CESQLParserParserRULE_functionIdentifier

	return p
}

func (s *FunctionIdentifierContext) GetParser() antlr.Parser { return s.parser }

func (s *FunctionIdentifierContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserIDENTIFIER, 0)
}

func (s *FunctionIdentifierContext) FUNCTION_IDENTIFIER_WITH_UNDERSCORE() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserFUNCTION_IDENTIFIER_WITH_UNDERSCORE, 0)
}

func (s *FunctionIdentifierContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *FunctionIdentifierContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *FunctionIdentifierContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitFunctionIdentifier(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CESQLParserParser) FunctionIdentifier() (localctx IFunctionIdentifierContext) {
	localctx = NewFunctionIdentifierContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 8, CESQLParserParserRULE_functionIdentifier)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(77)
		_la = p.GetTokenStream().LA(1)

		if !(_la == CESQLParserParserIDENTIFIER || _la == CESQLParserParserFUNCTION_IDENTIFIER_WITH_UNDERSCORE) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}

	return localctx
}

// IBooleanLiteralContext is an interface to support dynamic dispatch.
type IBooleanLiteralContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsBooleanLiteralContext differentiates from other interfaces.
	IsBooleanLiteralContext()
}

type BooleanLiteralContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyBooleanLiteralContext() *BooleanLiteralContext {
	var p = new(BooleanLiteralContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = CESQLParserParserRULE_booleanLiteral
	return p
}

func (*BooleanLiteralContext) IsBooleanLiteralContext() {}

func NewBooleanLiteralContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *BooleanLiteralContext {
	var p = new(BooleanLiteralContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = CESQLParserParserRULE_booleanLiteral

	return p
}

func (s *BooleanLiteralContext) GetParser() antlr.Parser { return s.parser }

func (s *BooleanLiteralContext) TRUE() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserTRUE, 0)
}

func (s *BooleanLiteralContext) FALSE() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserFALSE, 0)
}

func (s *BooleanLiteralContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *BooleanLiteralContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *BooleanLiteralContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitBooleanLiteral(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CESQLParserParser) BooleanLiteral() (localctx IBooleanLiteralContext) {
	localctx = NewBooleanLiteralContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 10, CESQLParserParserRULE_booleanLiteral)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(79)
		_la = p.GetTokenStream().LA(1)

		if !(_la == CESQLParserParserTRUE || _la == CESQLParserParserFALSE) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}

	return localctx
}

// IStringLiteralContext is an interface to support dynamic dispatch.
type IStringLiteralContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsStringLiteralContext differentiates from other interfaces.
	IsStringLiteralContext()
}

type StringLiteralContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyStringLiteralContext() *StringLiteralContext {
	var p = new(StringLiteralContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = CESQLParserParserRULE_stringLiteral
	return p
}

func (*StringLiteralContext) IsStringLiteralContext() {}

func NewStringLiteralContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *StringLiteralContext {
	var p = new(StringLiteralContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = CESQLParserParserRULE_stringLiteral

	return p
}

func (s *StringLiteralContext) GetParser() antlr.Parser { return s.parser }

func (s *StringLiteralContext) DQUOTED_STRING_LITERAL() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserDQUOTED_STRING_LITERAL, 0)
}

func (s *StringLiteralContext) SQUOTED_STRING_LITERAL() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserSQUOTED_STRING_LITERAL, 0)
}

func (s *StringLiteralContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *StringLiteralContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *StringLiteralContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitStringLiteral(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CESQLParserParser) StringLiteral() (localctx IStringLiteralContext) {
	localctx = NewStringLiteralContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 12, CESQLParserParserRULE_stringLiteral)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(81)
		_la = p.GetTokenStream().LA(1)

		if !(_la == CESQLParserParserDQUOTED_STRING_LITERAL || _la == CESQLParserParserSQUOTED_STRING_LITERAL) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}

	return localctx
}

// IIntegerLiteralContext is an interface to support dynamic dispatch.
type IIntegerLiteralContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsIntegerLiteralContext differentiates from other interfaces.
	IsIntegerLiteralContext()
}

type IntegerLiteralContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyIntegerLiteralContext() *IntegerLiteralContext {
	var p = new(IntegerLiteralContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = CESQLParserParserRULE_integerLiteral
	return p
}

func (*IntegerLiteralContext) IsIntegerLiteralContext() {}

func NewIntegerLiteralContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *IntegerLiteralContext {
	var p = new(IntegerLiteralContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = CESQLParserParserRULE_integerLiteral

	return p
}

func (s *IntegerLiteralContext) GetParser() antlr.Parser { return s.parser }

func (s *IntegerLiteralContext) INTEGER_LITERAL() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserINTEGER_LITERAL, 0)
}

func (s *IntegerLiteralContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *IntegerLiteralContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *IntegerLiteralContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitIntegerLiteral(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CESQLParserParser) IntegerLiteral() (localctx IIntegerLiteralContext) {
	localctx = NewIntegerLiteralContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 14, CESQLParserParserRULE_integerLiteral)

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(83)
		p.Match(CESQLParserParserINTEGER_LITERAL)
	}

	return localctx
}

// IFunctionParameterListContext is an interface to support dynamic dispatch.
type IFunctionParameterListContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsFunctionParameterListContext differentiates from other interfaces.
	IsFunctionParameterListContext()
}

type FunctionParameterListContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyFunctionParameterListContext() *FunctionParameterListContext {
	var p = new(FunctionParameterListContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = CESQLParserParserRULE_functionParameterList
	return p
}

func (*FunctionParameterListContext) IsFunctionParameterListContext() {}

func NewFunctionParameterListContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *FunctionParameterListContext {
	var p = new(FunctionParameterListContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = CESQLParserParserRULE_functionParameterList

	return p
}

func (s *FunctionParameterListContext) GetParser() antlr.Parser { return s.parser }

func (s *FunctionParameterListContext) LR_BRACKET() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserLR_BRACKET, 0)
}

func (s *FunctionParameterListContext) RR_BRACKET() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserRR_BRACKET, 0)
}

func (s *FunctionParameterListContext) AllExpression() []IExpressionContext {
	var ts = s.GetTypedRuleContexts(reflect.TypeOf((*IExpressionContext)(nil)).Elem())
	var tst = make([]IExpressionContext, len(ts))

	for i, t := range ts {
		if t != nil {
			tst[i] = t.(IExpressionContext)
		}
	}

	return tst
}

func (s *FunctionParameterListContext) Expression(i int) IExpressionContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IExpressionContext)(nil)).Elem(), i)

	if t == nil {
		return nil
	}

	return t.(IExpressionContext)
}

func (s *FunctionParameterListContext) AllCOMMA() []antlr.TerminalNode {
	return s.GetTokens(CESQLParserParserCOMMA)
}

func (s *FunctionParameterListContext) COMMA(i int) antlr.TerminalNode {
	return s.GetToken(CESQLParserParserCOMMA, i)
}

func (s *FunctionParameterListContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *FunctionParameterListContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *FunctionParameterListContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitFunctionParameterList(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CESQLParserParser) FunctionParameterList() (localctx IFunctionParameterListContext) {
	localctx = NewFunctionParameterListContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 16, CESQLParserParserRULE_functionParameterList)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(85)
		p.Match(CESQLParserParserLR_BRACKET)
	}
	p.SetState(94)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	if ((_la-2)&-(0x1f+1)) == 0 && ((int64(1)<<uint((_la-2)))&((int64(1)<<(CESQLParserParserLR_BRACKET-2))|(int64(1)<<(CESQLParserParserNOT-2))|(int64(1)<<(CESQLParserParserMINUS-2))|(int64(1)<<(CESQLParserParserEXISTS-2))|(int64(1)<<(CESQLParserParserTRUE-2))|(int64(1)<<(CESQLParserParserFALSE-2))|(int64(1)<<(CESQLParserParserDQUOTED_STRING_LITERAL-2))|(int64(1)<<(CESQLParserParserSQUOTED_STRING_LITERAL-2))|(int64(1)<<(CESQLParserParserINTEGER_LITERAL-2))|(int64(1)<<(CESQLParserParserIDENTIFIER-2))|(int64(1)<<(CESQLParserParserIDENTIFIER_WITH_NUMBER-2))|(int64(1)<<(CESQLParserParserFUNCTION_IDENTIFIER_WITH_UNDERSCORE-2)))) != 0 {
		{
			p.SetState(86)
			p.expression(0)
		}
		p.SetState(91)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)

		for _la == CESQLParserParserCOMMA {
			{
				p.SetState(87)
				p.Match(CESQLParserParserCOMMA)
			}
			{
				p.SetState(88)
				p.expression(0)
			}

			p.SetState(93)
			p.GetErrorHandler().Sync(p)
			_la = p.GetTokenStream().LA(1)
		}

	}
	{
		p.SetState(96)
		p.Match(CESQLParserParserRR_BRACKET)
	}

	return localctx
}

// ISetExpressionContext is an interface to support dynamic dispatch.
type ISetExpressionContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// IsSetExpressionContext differentiates from other interfaces.
	IsSetExpressionContext()
}

type SetExpressionContext struct {
	*antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptySetExpressionContext() *SetExpressionContext {
	var p = new(SetExpressionContext)
	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(nil, -1)
	p.RuleIndex = CESQLParserParserRULE_setExpression
	return p
}

func (*SetExpressionContext) IsSetExpressionContext() {}

func NewSetExpressionContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *SetExpressionContext {
	var p = new(SetExpressionContext)

	p.BaseParserRuleContext = antlr.NewBaseParserRuleContext(parent, invokingState)

	p.parser = parser
	p.RuleIndex = CESQLParserParserRULE_setExpression

	return p
}

func (s *SetExpressionContext) GetParser() antlr.Parser { return s.parser }

func (s *SetExpressionContext) LR_BRACKET() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserLR_BRACKET, 0)
}

func (s *SetExpressionContext) AllExpression() []IExpressionContext {
	var ts = s.GetTypedRuleContexts(reflect.TypeOf((*IExpressionContext)(nil)).Elem())
	var tst = make([]IExpressionContext, len(ts))

	for i, t := range ts {
		if t != nil {
			tst[i] = t.(IExpressionContext)
		}
	}

	return tst
}

func (s *SetExpressionContext) Expression(i int) IExpressionContext {
	var t = s.GetTypedRuleContext(reflect.TypeOf((*IExpressionContext)(nil)).Elem(), i)

	if t == nil {
		return nil
	}

	return t.(IExpressionContext)
}

func (s *SetExpressionContext) RR_BRACKET() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserRR_BRACKET, 0)
}

func (s *SetExpressionContext) AllCOMMA() []antlr.TerminalNode {
	return s.GetTokens(CESQLParserParserCOMMA)
}

func (s *SetExpressionContext) COMMA(i int) antlr.TerminalNode {
	return s.GetToken(CESQLParserParserCOMMA, i)
}

func (s *SetExpressionContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *SetExpressionContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *SetExpressionContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case CESQLParserVisitor:
		return t.VisitSetExpression(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CESQLParserParser) SetExpression() (localctx ISetExpressionContext) {
	localctx = NewSetExpressionContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 18, CESQLParserParserRULE_setExpression)
	var _la int

	defer func() {
		p.ExitRule()
	}()

	defer func() {
		if err := recover(); err != nil {
			if v, ok := err.(antlr.RecognitionException); ok {
				localctx.SetException(v)
				p.GetErrorHandler().ReportError(p, v)
				p.GetErrorHandler().Recover(p, v)
			} else {
				panic(err)
			}
		}
	}()

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(98)
		p.Match(CESQLParserParserLR_BRACKET)
	}
	{
		p.SetState(99)
		p.expression(0)
	}
	p.SetState(104)
	p.GetErrorHandler().Sync(p)
	_la = p.GetTokenStream().LA(1)

	for _la == CESQLParserParserCOMMA {
		{
			p.SetState(100)
			p.Match(CESQLParserParserCOMMA)
		}
		{
			p.SetState(101)
			p.expression(0)
		}

		p.SetState(106)
		p.GetErrorHandler().Sync(p)
		_la = p.GetTokenStream().LA(1)
	}
	{
		p.SetState(107)
		p.Match(CESQLParserParserRR_BRACKET)
	}

	return localctx
}

func (p *CESQLParserParser) Sempred(localctx antlr.RuleContext, ruleIndex, predIndex int) bool {
	switch ruleIndex {
	case 1:
		var t *ExpressionContext = nil
		if localctx != nil {
			t = localctx.(*ExpressionContext)
		}
		return p.Expression_Sempred(t, predIndex)

	default:
		panic("No predicate with index: " + fmt.Sprint(ruleIndex))
	}
}

func (p *CESQLParserParser) Expression_Sempred(localctx antlr.RuleContext, predIndex int) bool {
	switch predIndex {
	case 0:
		return p.Precpred(p.GetParserRuleContext(), 6)

	case 1:
		return p.Precpred(p.GetParserRuleContext(), 5)

	case 2:
		return p.Precpred(p.GetParserRuleContext(), 4)

	case 3:
		return p.Precpred(p.GetParserRuleContext(), 3)

	case 4:
		return p.Precpred(p.GetParserRuleContext(), 9)

	case 5:
		return p.Precpred(p.GetParserRuleContext(), 7)

	default:
		panic("No predicate with index: " + fmt.Sprint(predIndex))
	}
}
