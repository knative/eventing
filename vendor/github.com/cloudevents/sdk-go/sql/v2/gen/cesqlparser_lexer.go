/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

// Code generated from CESQLParser.g4 by ANTLR 4.9. DO NOT EDIT.

package gen

import (
	"fmt"
	"unicode"

	"github.com/antlr/antlr4/runtime/Go/antlr"
)

// Suppress unused import error
var _ = fmt.Printf
var _ = unicode.IsLetter

var serializedLexerAtn = []uint16{
	3, 24715, 42794, 33075, 47597, 16764, 15335, 30598, 22884, 2, 35, 237,
	8, 1, 4, 2, 9, 2, 4, 3, 9, 3, 4, 4, 9, 4, 4, 5, 9, 5, 4, 6, 9, 6, 4, 7,
	9, 7, 4, 8, 9, 8, 4, 9, 9, 9, 4, 10, 9, 10, 4, 11, 9, 11, 4, 12, 9, 12,
	4, 13, 9, 13, 4, 14, 9, 14, 4, 15, 9, 15, 4, 16, 9, 16, 4, 17, 9, 17, 4,
	18, 9, 18, 4, 19, 9, 19, 4, 20, 9, 20, 4, 21, 9, 21, 4, 22, 9, 22, 4, 23,
	9, 23, 4, 24, 9, 24, 4, 25, 9, 25, 4, 26, 9, 26, 4, 27, 9, 27, 4, 28, 9,
	28, 4, 29, 9, 29, 4, 30, 9, 30, 4, 31, 9, 31, 4, 32, 9, 32, 4, 33, 9, 33,
	4, 34, 9, 34, 4, 35, 9, 35, 4, 36, 9, 36, 4, 37, 9, 37, 4, 38, 9, 38, 4,
	39, 9, 39, 4, 40, 9, 40, 3, 2, 6, 2, 83, 10, 2, 13, 2, 14, 2, 84, 3, 2,
	3, 2, 3, 3, 6, 3, 90, 10, 3, 13, 3, 14, 3, 91, 3, 4, 3, 4, 3, 4, 3, 4,
	3, 4, 3, 4, 7, 4, 100, 10, 4, 12, 4, 14, 4, 103, 11, 4, 3, 4, 3, 4, 3,
	5, 3, 5, 3, 5, 3, 5, 3, 5, 3, 5, 7, 5, 113, 10, 5, 12, 5, 14, 5, 116, 11,
	5, 3, 5, 3, 5, 3, 6, 3, 6, 3, 7, 3, 7, 7, 7, 124, 10, 7, 12, 7, 14, 7,
	127, 11, 7, 3, 8, 3, 8, 3, 9, 3, 9, 3, 10, 3, 10, 3, 11, 3, 11, 3, 12,
	3, 12, 3, 13, 3, 13, 5, 13, 141, 10, 13, 3, 14, 3, 14, 3, 14, 3, 14, 3,
	15, 3, 15, 3, 15, 3, 16, 3, 16, 3, 16, 3, 16, 3, 17, 3, 17, 3, 17, 3, 17,
	3, 18, 3, 18, 3, 19, 3, 19, 3, 20, 3, 20, 3, 21, 3, 21, 3, 22, 3, 22, 3,
	23, 3, 23, 3, 24, 3, 24, 3, 24, 3, 25, 3, 25, 3, 26, 3, 26, 3, 26, 3, 27,
	3, 27, 3, 28, 3, 28, 3, 28, 3, 29, 3, 29, 3, 29, 3, 30, 3, 30, 3, 30, 3,
	30, 3, 30, 3, 31, 3, 31, 3, 31, 3, 31, 3, 31, 3, 31, 3, 31, 3, 32, 3, 32,
	3, 32, 3, 33, 3, 33, 3, 33, 3, 33, 3, 33, 3, 34, 3, 34, 3, 34, 3, 34, 3,
	34, 3, 34, 3, 35, 3, 35, 3, 36, 3, 36, 3, 37, 6, 37, 217, 10, 37, 13, 37,
	14, 37, 218, 3, 38, 6, 38, 222, 10, 38, 13, 38, 14, 38, 223, 3, 39, 6,
	39, 227, 10, 39, 13, 39, 14, 39, 228, 3, 40, 3, 40, 7, 40, 233, 10, 40,
	12, 40, 14, 40, 236, 11, 40, 2, 2, 41, 3, 3, 5, 2, 7, 2, 9, 2, 11, 2, 13,
	2, 15, 4, 17, 5, 19, 6, 21, 7, 23, 8, 25, 2, 27, 9, 29, 10, 31, 11, 33,
	12, 35, 13, 37, 14, 39, 15, 41, 16, 43, 17, 45, 18, 47, 19, 49, 20, 51,
	21, 53, 22, 55, 23, 57, 24, 59, 25, 61, 26, 63, 27, 65, 28, 67, 29, 69,
	30, 71, 31, 73, 32, 75, 33, 77, 34, 79, 35, 3, 2, 10, 5, 2, 11, 12, 15,
	15, 34, 34, 5, 2, 50, 59, 67, 92, 99, 124, 4, 2, 36, 36, 94, 94, 4, 2,
	41, 41, 94, 94, 3, 2, 50, 59, 3, 2, 67, 92, 4, 2, 67, 92, 97, 97, 4, 2,
	67, 92, 99, 124, 2, 244, 2, 3, 3, 2, 2, 2, 2, 15, 3, 2, 2, 2, 2, 17, 3,
	2, 2, 2, 2, 19, 3, 2, 2, 2, 2, 21, 3, 2, 2, 2, 2, 23, 3, 2, 2, 2, 2, 27,
	3, 2, 2, 2, 2, 29, 3, 2, 2, 2, 2, 31, 3, 2, 2, 2, 2, 33, 3, 2, 2, 2, 2,
	35, 3, 2, 2, 2, 2, 37, 3, 2, 2, 2, 2, 39, 3, 2, 2, 2, 2, 41, 3, 2, 2, 2,
	2, 43, 3, 2, 2, 2, 2, 45, 3, 2, 2, 2, 2, 47, 3, 2, 2, 2, 2, 49, 3, 2, 2,
	2, 2, 51, 3, 2, 2, 2, 2, 53, 3, 2, 2, 2, 2, 55, 3, 2, 2, 2, 2, 57, 3, 2,
	2, 2, 2, 59, 3, 2, 2, 2, 2, 61, 3, 2, 2, 2, 2, 63, 3, 2, 2, 2, 2, 65, 3,
	2, 2, 2, 2, 67, 3, 2, 2, 2, 2, 69, 3, 2, 2, 2, 2, 71, 3, 2, 2, 2, 2, 73,
	3, 2, 2, 2, 2, 75, 3, 2, 2, 2, 2, 77, 3, 2, 2, 2, 2, 79, 3, 2, 2, 2, 3,
	82, 3, 2, 2, 2, 5, 89, 3, 2, 2, 2, 7, 93, 3, 2, 2, 2, 9, 106, 3, 2, 2,
	2, 11, 119, 3, 2, 2, 2, 13, 121, 3, 2, 2, 2, 15, 128, 3, 2, 2, 2, 17, 130,
	3, 2, 2, 2, 19, 132, 3, 2, 2, 2, 21, 134, 3, 2, 2, 2, 23, 136, 3, 2, 2,
	2, 25, 140, 3, 2, 2, 2, 27, 142, 3, 2, 2, 2, 29, 146, 3, 2, 2, 2, 31, 149,
	3, 2, 2, 2, 33, 153, 3, 2, 2, 2, 35, 157, 3, 2, 2, 2, 37, 159, 3, 2, 2,
	2, 39, 161, 3, 2, 2, 2, 41, 163, 3, 2, 2, 2, 43, 165, 3, 2, 2, 2, 45, 167,
	3, 2, 2, 2, 47, 169, 3, 2, 2, 2, 49, 172, 3, 2, 2, 2, 51, 174, 3, 2, 2,
	2, 53, 177, 3, 2, 2, 2, 55, 179, 3, 2, 2, 2, 57, 182, 3, 2, 2, 2, 59, 185,
	3, 2, 2, 2, 61, 190, 3, 2, 2, 2, 63, 197, 3, 2, 2, 2, 65, 200, 3, 2, 2,
	2, 67, 205, 3, 2, 2, 2, 69, 211, 3, 2, 2, 2, 71, 213, 3, 2, 2, 2, 73, 216,
	3, 2, 2, 2, 75, 221, 3, 2, 2, 2, 77, 226, 3, 2, 2, 2, 79, 230, 3, 2, 2,
	2, 81, 83, 9, 2, 2, 2, 82, 81, 3, 2, 2, 2, 83, 84, 3, 2, 2, 2, 84, 82,
	3, 2, 2, 2, 84, 85, 3, 2, 2, 2, 85, 86, 3, 2, 2, 2, 86, 87, 8, 2, 2, 2,
	87, 4, 3, 2, 2, 2, 88, 90, 9, 3, 2, 2, 89, 88, 3, 2, 2, 2, 90, 91, 3, 2,
	2, 2, 91, 89, 3, 2, 2, 2, 91, 92, 3, 2, 2, 2, 92, 6, 3, 2, 2, 2, 93, 101,
	7, 36, 2, 2, 94, 95, 7, 94, 2, 2, 95, 100, 11, 2, 2, 2, 96, 97, 7, 36,
	2, 2, 97, 100, 7, 36, 2, 2, 98, 100, 10, 4, 2, 2, 99, 94, 3, 2, 2, 2, 99,
	96, 3, 2, 2, 2, 99, 98, 3, 2, 2, 2, 100, 103, 3, 2, 2, 2, 101, 99, 3, 2,
	2, 2, 101, 102, 3, 2, 2, 2, 102, 104, 3, 2, 2, 2, 103, 101, 3, 2, 2, 2,
	104, 105, 7, 36, 2, 2, 105, 8, 3, 2, 2, 2, 106, 114, 7, 41, 2, 2, 107,
	108, 7, 94, 2, 2, 108, 113, 11, 2, 2, 2, 109, 110, 7, 41, 2, 2, 110, 113,
	7, 41, 2, 2, 111, 113, 10, 5, 2, 2, 112, 107, 3, 2, 2, 2, 112, 109, 3,
	2, 2, 2, 112, 111, 3, 2, 2, 2, 113, 116, 3, 2, 2, 2, 114, 112, 3, 2, 2,
	2, 114, 115, 3, 2, 2, 2, 115, 117, 3, 2, 2, 2, 116, 114, 3, 2, 2, 2, 117,
	118, 7, 41, 2, 2, 118, 10, 3, 2, 2, 2, 119, 120, 9, 6, 2, 2, 120, 12, 3,
	2, 2, 2, 121, 125, 9, 7, 2, 2, 122, 124, 9, 8, 2, 2, 123, 122, 3, 2, 2,
	2, 124, 127, 3, 2, 2, 2, 125, 123, 3, 2, 2, 2, 125, 126, 3, 2, 2, 2, 126,
	14, 3, 2, 2, 2, 127, 125, 3, 2, 2, 2, 128, 129, 7, 42, 2, 2, 129, 16, 3,
	2, 2, 2, 130, 131, 7, 43, 2, 2, 131, 18, 3, 2, 2, 2, 132, 133, 7, 46, 2,
	2, 133, 20, 3, 2, 2, 2, 134, 135, 7, 41, 2, 2, 135, 22, 3, 2, 2, 2, 136,
	137, 7, 36, 2, 2, 137, 24, 3, 2, 2, 2, 138, 141, 5, 21, 11, 2, 139, 141,
	5, 23, 12, 2, 140, 138, 3, 2, 2, 2, 140, 139, 3, 2, 2, 2, 141, 26, 3, 2,
	2, 2, 142, 143, 7, 67, 2, 2, 143, 144, 7, 80, 2, 2, 144, 145, 7, 70, 2,
	2, 145, 28, 3, 2, 2, 2, 146, 147, 7, 81, 2, 2, 147, 148, 7, 84, 2, 2, 148,
	30, 3, 2, 2, 2, 149, 150, 7, 90, 2, 2, 150, 151, 7, 81, 2, 2, 151, 152,
	7, 84, 2, 2, 152, 32, 3, 2, 2, 2, 153, 154, 7, 80, 2, 2, 154, 155, 7, 81,
	2, 2, 155, 156, 7, 86, 2, 2, 156, 34, 3, 2, 2, 2, 157, 158, 7, 44, 2, 2,
	158, 36, 3, 2, 2, 2, 159, 160, 7, 49, 2, 2, 160, 38, 3, 2, 2, 2, 161, 162,
	7, 39, 2, 2, 162, 40, 3, 2, 2, 2, 163, 164, 7, 45, 2, 2, 164, 42, 3, 2,
	2, 2, 165, 166, 7, 47, 2, 2, 166, 44, 3, 2, 2, 2, 167, 168, 7, 63, 2, 2,
	168, 46, 3, 2, 2, 2, 169, 170, 7, 35, 2, 2, 170, 171, 7, 63, 2, 2, 171,
	48, 3, 2, 2, 2, 172, 173, 7, 64, 2, 2, 173, 50, 3, 2, 2, 2, 174, 175, 7,
	64, 2, 2, 175, 176, 7, 63, 2, 2, 176, 52, 3, 2, 2, 2, 177, 178, 7, 62,
	2, 2, 178, 54, 3, 2, 2, 2, 179, 180, 7, 62, 2, 2, 180, 181, 7, 64, 2, 2,
	181, 56, 3, 2, 2, 2, 182, 183, 7, 62, 2, 2, 183, 184, 7, 63, 2, 2, 184,
	58, 3, 2, 2, 2, 185, 186, 7, 78, 2, 2, 186, 187, 7, 75, 2, 2, 187, 188,
	7, 77, 2, 2, 188, 189, 7, 71, 2, 2, 189, 60, 3, 2, 2, 2, 190, 191, 7, 71,
	2, 2, 191, 192, 7, 90, 2, 2, 192, 193, 7, 75, 2, 2, 193, 194, 7, 85, 2,
	2, 194, 195, 7, 86, 2, 2, 195, 196, 7, 85, 2, 2, 196, 62, 3, 2, 2, 2, 197,
	198, 7, 75, 2, 2, 198, 199, 7, 80, 2, 2, 199, 64, 3, 2, 2, 2, 200, 201,
	7, 86, 2, 2, 201, 202, 7, 84, 2, 2, 202, 203, 7, 87, 2, 2, 203, 204, 7,
	71, 2, 2, 204, 66, 3, 2, 2, 2, 205, 206, 7, 72, 2, 2, 206, 207, 7, 67,
	2, 2, 207, 208, 7, 78, 2, 2, 208, 209, 7, 85, 2, 2, 209, 210, 7, 71, 2,
	2, 210, 68, 3, 2, 2, 2, 211, 212, 5, 7, 4, 2, 212, 70, 3, 2, 2, 2, 213,
	214, 5, 9, 5, 2, 214, 72, 3, 2, 2, 2, 215, 217, 5, 11, 6, 2, 216, 215,
	3, 2, 2, 2, 217, 218, 3, 2, 2, 2, 218, 216, 3, 2, 2, 2, 218, 219, 3, 2,
	2, 2, 219, 74, 3, 2, 2, 2, 220, 222, 9, 9, 2, 2, 221, 220, 3, 2, 2, 2,
	222, 223, 3, 2, 2, 2, 223, 221, 3, 2, 2, 2, 223, 224, 3, 2, 2, 2, 224,
	76, 3, 2, 2, 2, 225, 227, 9, 3, 2, 2, 226, 225, 3, 2, 2, 2, 227, 228, 3,
	2, 2, 2, 228, 226, 3, 2, 2, 2, 228, 229, 3, 2, 2, 2, 229, 78, 3, 2, 2,
	2, 230, 234, 9, 7, 2, 2, 231, 233, 9, 8, 2, 2, 232, 231, 3, 2, 2, 2, 233,
	236, 3, 2, 2, 2, 234, 232, 3, 2, 2, 2, 234, 235, 3, 2, 2, 2, 235, 80, 3,
	2, 2, 2, 236, 234, 3, 2, 2, 2, 15, 2, 84, 91, 99, 101, 112, 114, 125, 140,
	218, 223, 228, 234, 3, 8, 2, 2,
}

var lexerChannelNames = []string{
	"DEFAULT_TOKEN_CHANNEL", "HIDDEN",
}

var lexerModeNames = []string{
	"DEFAULT_MODE",
}

var lexerLiteralNames = []string{
	"", "", "'('", "')'", "','", "'''", "'\"'", "'AND'", "'OR'", "'XOR'", "'NOT'",
	"'*'", "'/'", "'%'", "'+'", "'-'", "'='", "'!='", "'>'", "'>='", "'<'",
	"'<>'", "'<='", "'LIKE'", "'EXISTS'", "'IN'", "'TRUE'", "'FALSE'",
}

var lexerSymbolicNames = []string{
	"", "SPACE", "LR_BRACKET", "RR_BRACKET", "COMMA", "SINGLE_QUOTE_SYMB",
	"DOUBLE_QUOTE_SYMB", "AND", "OR", "XOR", "NOT", "STAR", "DIVIDE", "MODULE",
	"PLUS", "MINUS", "EQUAL", "NOT_EQUAL", "GREATER", "GREATER_OR_EQUAL", "LESS",
	"LESS_GREATER", "LESS_OR_EQUAL", "LIKE", "EXISTS", "IN", "TRUE", "FALSE",
	"DQUOTED_STRING_LITERAL", "SQUOTED_STRING_LITERAL", "INTEGER_LITERAL",
	"IDENTIFIER", "IDENTIFIER_WITH_NUMBER", "FUNCTION_IDENTIFIER_WITH_UNDERSCORE",
}

var lexerRuleNames = []string{
	"SPACE", "ID_LITERAL", "DQUOTA_STRING", "SQUOTA_STRING", "INT_DIGIT", "FN_LITERAL",
	"LR_BRACKET", "RR_BRACKET", "COMMA", "SINGLE_QUOTE_SYMB", "DOUBLE_QUOTE_SYMB",
	"QUOTE_SYMB", "AND", "OR", "XOR", "NOT", "STAR", "DIVIDE", "MODULE", "PLUS",
	"MINUS", "EQUAL", "NOT_EQUAL", "GREATER", "GREATER_OR_EQUAL", "LESS", "LESS_GREATER",
	"LESS_OR_EQUAL", "LIKE", "EXISTS", "IN", "TRUE", "FALSE", "DQUOTED_STRING_LITERAL",
	"SQUOTED_STRING_LITERAL", "INTEGER_LITERAL", "IDENTIFIER", "IDENTIFIER_WITH_NUMBER",
	"FUNCTION_IDENTIFIER_WITH_UNDERSCORE",
}

type CESQLParserLexer struct {
	*antlr.BaseLexer
	channelNames []string
	modeNames    []string
	// TODO: EOF string
}

// NewCESQLParserLexer produces a new lexer instance for the optional input antlr.CharStream.
//
// The *CESQLParserLexer instance produced may be reused by calling the SetInputStream method.
// The initial lexer configuration is expensive to construct, and the object is not thread-safe;
// however, if used within a Golang sync.Pool, the construction cost amortizes well and the
// objects can be used in a thread-safe manner.
func NewCESQLParserLexer(input antlr.CharStream) *CESQLParserLexer {
	l := new(CESQLParserLexer)
	lexerDeserializer := antlr.NewATNDeserializer(nil)
	lexerAtn := lexerDeserializer.DeserializeFromUInt16(serializedLexerAtn)
	lexerDecisionToDFA := make([]*antlr.DFA, len(lexerAtn.DecisionToState))
	for index, ds := range lexerAtn.DecisionToState {
		lexerDecisionToDFA[index] = antlr.NewDFA(ds, index)
	}
	l.BaseLexer = antlr.NewBaseLexer(input)
	l.Interpreter = antlr.NewLexerATNSimulator(l, lexerAtn, lexerDecisionToDFA, antlr.NewPredictionContextCache())

	l.channelNames = lexerChannelNames
	l.modeNames = lexerModeNames
	l.RuleNames = lexerRuleNames
	l.LiteralNames = lexerLiteralNames
	l.SymbolicNames = lexerSymbolicNames
	l.GrammarFileName = "CESQLParser.g4"
	// TODO: l.EOF = antlr.TokenEOF

	return l
}

// CESQLParserLexer tokens.
const (
	CESQLParserLexerSPACE                               = 1
	CESQLParserLexerLR_BRACKET                          = 2
	CESQLParserLexerRR_BRACKET                          = 3
	CESQLParserLexerCOMMA                               = 4
	CESQLParserLexerSINGLE_QUOTE_SYMB                   = 5
	CESQLParserLexerDOUBLE_QUOTE_SYMB                   = 6
	CESQLParserLexerAND                                 = 7
	CESQLParserLexerOR                                  = 8
	CESQLParserLexerXOR                                 = 9
	CESQLParserLexerNOT                                 = 10
	CESQLParserLexerSTAR                                = 11
	CESQLParserLexerDIVIDE                              = 12
	CESQLParserLexerMODULE                              = 13
	CESQLParserLexerPLUS                                = 14
	CESQLParserLexerMINUS                               = 15
	CESQLParserLexerEQUAL                               = 16
	CESQLParserLexerNOT_EQUAL                           = 17
	CESQLParserLexerGREATER                             = 18
	CESQLParserLexerGREATER_OR_EQUAL                    = 19
	CESQLParserLexerLESS                                = 20
	CESQLParserLexerLESS_GREATER                        = 21
	CESQLParserLexerLESS_OR_EQUAL                       = 22
	CESQLParserLexerLIKE                                = 23
	CESQLParserLexerEXISTS                              = 24
	CESQLParserLexerIN                                  = 25
	CESQLParserLexerTRUE                                = 26
	CESQLParserLexerFALSE                               = 27
	CESQLParserLexerDQUOTED_STRING_LITERAL              = 28
	CESQLParserLexerSQUOTED_STRING_LITERAL              = 29
	CESQLParserLexerINTEGER_LITERAL                     = 30
	CESQLParserLexerIDENTIFIER                          = 31
	CESQLParserLexerIDENTIFIER_WITH_NUMBER              = 32
	CESQLParserLexerFUNCTION_IDENTIFIER_WITH_UNDERSCORE = 33
)
