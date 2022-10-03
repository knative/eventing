// Code generated from CESQLParser.g4 by ANTLR 4.10.1. DO NOT EDIT.

package gen // CESQLParser
import (
	"fmt"
	"strconv"
	"sync"

	"github.com/antlr/antlr4/runtime/Go/antlr"
)

// Suppress unused import errors
var _ = fmt.Printf
var _ = strconv.Itoa
var _ = sync.Once{}

type CESQLParserParser struct {
	*antlr.BaseParser
}

var cesqlparserParserStaticData struct {
	once                   sync.Once
	serializedATN          []int32
	literalNames           []string
	symbolicNames          []string
	ruleNames              []string
	predictionContextCache *antlr.PredictionContextCache
	atn                    *antlr.ATN
	decisionToDFA          []*antlr.DFA
}

func cesqlparserParserInit() {
	staticData := &cesqlparserParserStaticData
	staticData.literalNames = []string{
		"", "", "'('", "')'", "','", "'''", "'\"'", "'AND'", "'OR'", "'XOR'",
		"'NOT'", "'*'", "'/'", "'%'", "'+'", "'-'", "'='", "'!='", "'>'", "'>='",
		"'<'", "'<>'", "'<='", "'LIKE'", "'EXISTS'", "'IN'", "'TRUE'", "'FALSE'",
	}
	staticData.symbolicNames = []string{
		"", "SPACE", "LR_BRACKET", "RR_BRACKET", "COMMA", "SINGLE_QUOTE_SYMB",
		"DOUBLE_QUOTE_SYMB", "AND", "OR", "XOR", "NOT", "STAR", "DIVIDE", "MODULE",
		"PLUS", "MINUS", "EQUAL", "NOT_EQUAL", "GREATER", "GREATER_OR_EQUAL",
		"LESS", "LESS_GREATER", "LESS_OR_EQUAL", "LIKE", "EXISTS", "IN", "TRUE",
		"FALSE", "DQUOTED_STRING_LITERAL", "SQUOTED_STRING_LITERAL", "INTEGER_LITERAL",
		"IDENTIFIER", "IDENTIFIER_WITH_NUMBER", "FUNCTION_IDENTIFIER_WITH_UNDERSCORE",
	}
	staticData.ruleNames = []string{
		"cesql", "expression", "atom", "identifier", "functionIdentifier", "booleanLiteral",
		"stringLiteral", "integerLiteral", "functionParameterList", "setExpression",
	}
	staticData.predictionContextCache = antlr.NewPredictionContextCache()
	staticData.serializedATN = []int32{
		4, 1, 33, 110, 2, 0, 7, 0, 2, 1, 7, 1, 2, 2, 7, 2, 2, 3, 7, 3, 2, 4, 7,
		4, 2, 5, 7, 5, 2, 6, 7, 6, 2, 7, 7, 7, 2, 8, 7, 8, 2, 9, 7, 9, 1, 0, 1,
		0, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 3, 1, 39, 8, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 3, 1, 55, 8, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 3, 1, 61, 8, 1, 1, 1, 1, 1, 5, 1, 65, 8, 1, 10, 1,
		12, 1, 68, 9, 1, 1, 2, 1, 2, 1, 2, 1, 2, 3, 2, 74, 8, 2, 1, 3, 1, 3, 1,
		4, 1, 4, 1, 5, 1, 5, 1, 6, 1, 6, 1, 7, 1, 7, 1, 8, 1, 8, 1, 8, 1, 8, 5,
		8, 90, 8, 8, 10, 8, 12, 8, 93, 9, 8, 3, 8, 95, 8, 8, 1, 8, 1, 8, 1, 9,
		1, 9, 1, 9, 1, 9, 5, 9, 103, 8, 9, 10, 9, 12, 9, 106, 9, 9, 1, 9, 1, 9,
		1, 9, 0, 1, 2, 10, 0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 0, 8, 1, 0, 11, 13,
		1, 0, 14, 15, 1, 0, 16, 22, 1, 0, 7, 9, 1, 0, 31, 32, 2, 0, 31, 31, 33,
		33, 1, 0, 26, 27, 1, 0, 28, 29, 118, 0, 20, 1, 0, 0, 0, 2, 38, 1, 0, 0,
		0, 4, 73, 1, 0, 0, 0, 6, 75, 1, 0, 0, 0, 8, 77, 1, 0, 0, 0, 10, 79, 1,
		0, 0, 0, 12, 81, 1, 0, 0, 0, 14, 83, 1, 0, 0, 0, 16, 85, 1, 0, 0, 0, 18,
		98, 1, 0, 0, 0, 20, 21, 3, 2, 1, 0, 21, 22, 5, 0, 0, 1, 22, 1, 1, 0, 0,
		0, 23, 24, 6, 1, -1, 0, 24, 25, 3, 8, 4, 0, 25, 26, 3, 16, 8, 0, 26, 39,
		1, 0, 0, 0, 27, 28, 5, 10, 0, 0, 28, 39, 3, 2, 1, 11, 29, 30, 5, 15, 0,
		0, 30, 39, 3, 2, 1, 10, 31, 32, 5, 24, 0, 0, 32, 39, 3, 6, 3, 0, 33, 34,
		5, 2, 0, 0, 34, 35, 3, 2, 1, 0, 35, 36, 5, 3, 0, 0, 36, 39, 1, 0, 0, 0,
		37, 39, 3, 4, 2, 0, 38, 23, 1, 0, 0, 0, 38, 27, 1, 0, 0, 0, 38, 29, 1,
		0, 0, 0, 38, 31, 1, 0, 0, 0, 38, 33, 1, 0, 0, 0, 38, 37, 1, 0, 0, 0, 39,
		66, 1, 0, 0, 0, 40, 41, 10, 6, 0, 0, 41, 42, 7, 0, 0, 0, 42, 65, 3, 2,
		1, 7, 43, 44, 10, 5, 0, 0, 44, 45, 7, 1, 0, 0, 45, 65, 3, 2, 1, 6, 46,
		47, 10, 4, 0, 0, 47, 48, 7, 2, 0, 0, 48, 65, 3, 2, 1, 5, 49, 50, 10, 3,
		0, 0, 50, 51, 7, 3, 0, 0, 51, 65, 3, 2, 1, 3, 52, 54, 10, 9, 0, 0, 53,
		55, 5, 10, 0, 0, 54, 53, 1, 0, 0, 0, 54, 55, 1, 0, 0, 0, 55, 56, 1, 0,
		0, 0, 56, 57, 5, 23, 0, 0, 57, 65, 3, 12, 6, 0, 58, 60, 10, 7, 0, 0, 59,
		61, 5, 10, 0, 0, 60, 59, 1, 0, 0, 0, 60, 61, 1, 0, 0, 0, 61, 62, 1, 0,
		0, 0, 62, 63, 5, 25, 0, 0, 63, 65, 3, 18, 9, 0, 64, 40, 1, 0, 0, 0, 64,
		43, 1, 0, 0, 0, 64, 46, 1, 0, 0, 0, 64, 49, 1, 0, 0, 0, 64, 52, 1, 0, 0,
		0, 64, 58, 1, 0, 0, 0, 65, 68, 1, 0, 0, 0, 66, 64, 1, 0, 0, 0, 66, 67,
		1, 0, 0, 0, 67, 3, 1, 0, 0, 0, 68, 66, 1, 0, 0, 0, 69, 74, 3, 10, 5, 0,
		70, 74, 3, 14, 7, 0, 71, 74, 3, 12, 6, 0, 72, 74, 3, 6, 3, 0, 73, 69, 1,
		0, 0, 0, 73, 70, 1, 0, 0, 0, 73, 71, 1, 0, 0, 0, 73, 72, 1, 0, 0, 0, 74,
		5, 1, 0, 0, 0, 75, 76, 7, 4, 0, 0, 76, 7, 1, 0, 0, 0, 77, 78, 7, 5, 0,
		0, 78, 9, 1, 0, 0, 0, 79, 80, 7, 6, 0, 0, 80, 11, 1, 0, 0, 0, 81, 82, 7,
		7, 0, 0, 82, 13, 1, 0, 0, 0, 83, 84, 5, 30, 0, 0, 84, 15, 1, 0, 0, 0, 85,
		94, 5, 2, 0, 0, 86, 91, 3, 2, 1, 0, 87, 88, 5, 4, 0, 0, 88, 90, 3, 2, 1,
		0, 89, 87, 1, 0, 0, 0, 90, 93, 1, 0, 0, 0, 91, 89, 1, 0, 0, 0, 91, 92,
		1, 0, 0, 0, 92, 95, 1, 0, 0, 0, 93, 91, 1, 0, 0, 0, 94, 86, 1, 0, 0, 0,
		94, 95, 1, 0, 0, 0, 95, 96, 1, 0, 0, 0, 96, 97, 5, 3, 0, 0, 97, 17, 1,
		0, 0, 0, 98, 99, 5, 2, 0, 0, 99, 104, 3, 2, 1, 0, 100, 101, 5, 4, 0, 0,
		101, 103, 3, 2, 1, 0, 102, 100, 1, 0, 0, 0, 103, 106, 1, 0, 0, 0, 104,
		102, 1, 0, 0, 0, 104, 105, 1, 0, 0, 0, 105, 107, 1, 0, 0, 0, 106, 104,
		1, 0, 0, 0, 107, 108, 5, 3, 0, 0, 108, 19, 1, 0, 0, 0, 9, 38, 54, 60, 64,
		66, 73, 91, 94, 104,
	}
	deserializer := antlr.NewATNDeserializer(nil)
	staticData.atn = deserializer.Deserialize(staticData.serializedATN)
	atn := staticData.atn
	staticData.decisionToDFA = make([]*antlr.DFA, len(atn.DecisionToState))
	decisionToDFA := staticData.decisionToDFA
	for index, state := range atn.DecisionToState {
		decisionToDFA[index] = antlr.NewDFA(state, index)
	}
}

// CESQLParserParserInit initializes any static state used to implement CESQLParserParser. By default the
// static state used to implement the parser is lazily initialized during the first call to
// NewCESQLParserParser(). You can call this function if you wish to initialize the static state ahead
// of time.
func CESQLParserParserInit() {
	staticData := &cesqlparserParserStaticData
	staticData.once.Do(cesqlparserParserInit)
}

// NewCESQLParserParser produces a new parser instance for the optional input antlr.TokenStream.
func NewCESQLParserParser(input antlr.TokenStream) *CESQLParserParser {
	CESQLParserParserInit()
	this := new(CESQLParserParser)
	this.BaseParser = antlr.NewBaseParser(input)
	staticData := &cesqlparserParserStaticData
	this.Interpreter = antlr.NewParserATNSimulator(this, staticData.atn, staticData.decisionToDFA, staticData.predictionContextCache)
	this.RuleNames = staticData.ruleNames
	this.LiteralNames = staticData.literalNames
	this.SymbolicNames = staticData.symbolicNames
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
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExpressionContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

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
	this := p
	_ = this

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
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExpressionContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExpressionContext)
}

func (s *InExpressionContext) IN() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserIN, 0)
}

func (s *InExpressionContext) SetExpression() ISetExpressionContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ISetExpressionContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

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
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IExpressionContext); ok {
			len++
		}
	}

	tst := make([]IExpressionContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IExpressionContext); ok {
			tst[i] = t.(IExpressionContext)
			i++
		}
	}

	return tst
}

func (s *BinaryComparisonExpressionContext) Expression(i int) IExpressionContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExpressionContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

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
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IAtomContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

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
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IIdentifierContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

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
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IExpressionContext); ok {
			len++
		}
	}

	tst := make([]IExpressionContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IExpressionContext); ok {
			tst[i] = t.(IExpressionContext)
			i++
		}
	}

	return tst
}

func (s *BinaryLogicExpressionContext) Expression(i int) IExpressionContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExpressionContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

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
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExpressionContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExpressionContext)
}

func (s *LikeExpressionContext) LIKE() antlr.TerminalNode {
	return s.GetToken(CESQLParserParserLIKE, 0)
}

func (s *LikeExpressionContext) StringLiteral() IStringLiteralContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IStringLiteralContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

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
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IFunctionIdentifierContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IFunctionIdentifierContext)
}

func (s *FunctionInvocationExpressionContext) FunctionParameterList() IFunctionParameterListContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IFunctionParameterListContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

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
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IExpressionContext); ok {
			len++
		}
	}

	tst := make([]IExpressionContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IExpressionContext); ok {
			tst[i] = t.(IExpressionContext)
			i++
		}
	}

	return tst
}

func (s *BinaryMultiplicativeExpressionContext) Expression(i int) IExpressionContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExpressionContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

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
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExpressionContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

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
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExpressionContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

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
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExpressionContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

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
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IExpressionContext); ok {
			len++
		}
	}

	tst := make([]IExpressionContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IExpressionContext); ok {
			tst[i] = t.(IExpressionContext)
			i++
		}
	}

	return tst
}

func (s *BinaryAdditiveExpressionContext) Expression(i int) IExpressionContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExpressionContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

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
	this := p
	_ = this

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
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IBooleanLiteralContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

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
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IIdentifierContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

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
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IStringLiteralContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

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
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IIntegerLiteralContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

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
	this := p
	_ = this

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
	this := p
	_ = this

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
	this := p
	_ = this

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
	this := p
	_ = this

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
	this := p
	_ = this

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
	this := p
	_ = this

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
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IExpressionContext); ok {
			len++
		}
	}

	tst := make([]IExpressionContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IExpressionContext); ok {
			tst[i] = t.(IExpressionContext)
			i++
		}
	}

	return tst
}

func (s *FunctionParameterListContext) Expression(i int) IExpressionContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExpressionContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

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
	this := p
	_ = this

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
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IExpressionContext); ok {
			len++
		}
	}

	tst := make([]IExpressionContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IExpressionContext); ok {
			tst[i] = t.(IExpressionContext)
			i++
		}
	}

	return tst
}

func (s *SetExpressionContext) Expression(i int) IExpressionContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExpressionContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

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
	this := p
	_ = this

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
	this := p
	_ = this

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
