/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package parser

import (
	"strconv"
	"strings"

	"github.com/antlr/antlr4/runtime/Go/antlr"

	cesql "github.com/cloudevents/sdk-go/sql/v2"
	"github.com/cloudevents/sdk-go/sql/v2/expression"
	"github.com/cloudevents/sdk-go/sql/v2/gen"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type expressionVisitor struct {
	parsingErrors []error
}

var _ gen.CESQLParserVisitor = (*expressionVisitor)(nil)

func NewExpressionVisitor() gen.CESQLParserVisitor {
	return &expressionVisitor{}
}

// antlr.ParseTreeVisitor implementation

func (v *expressionVisitor) Visit(tree antlr.ParseTree) interface{} {
	// If you're wondering why I had to manually implement this stuff:
	// https://github.com/antlr/antlr4/issues/2504
	switch tree.(type) {
	case *gen.CesqlContext:
		return v.VisitCesql(tree.(*gen.CesqlContext))
	case *gen.AtomExpressionContext:
		return v.VisitAtomExpression(tree.(*gen.AtomExpressionContext))
	case *gen.UnaryNumericExpressionContext:
		return v.VisitUnaryNumericExpression(tree.(*gen.UnaryNumericExpressionContext))
	case *gen.UnaryLogicExpressionContext:
		return v.VisitUnaryLogicExpression(tree.(*gen.UnaryLogicExpressionContext))
	case *gen.BooleanAtomContext:
		return v.VisitBooleanAtom(tree.(*gen.BooleanAtomContext))
	case *gen.BooleanLiteralContext:
		return v.VisitBooleanLiteral(tree.(*gen.BooleanLiteralContext))
	case *gen.IntegerAtomContext:
		return v.VisitIntegerAtom(tree.(*gen.IntegerAtomContext))
	case *gen.IntegerLiteralContext:
		return v.VisitIntegerLiteral(tree.(*gen.IntegerLiteralContext))
	case *gen.StringAtomContext:
		return v.VisitStringAtom(tree.(*gen.StringAtomContext))
	case *gen.StringLiteralContext:
		return v.VisitStringLiteral(tree.(*gen.StringLiteralContext))
	case *gen.ExistsExpressionContext:
		return v.VisitExistsExpression(tree.(*gen.ExistsExpressionContext))
	case *gen.InExpressionContext:
		return v.VisitInExpression(tree.(*gen.InExpressionContext))
	case *gen.IdentifierAtomContext:
		return v.VisitIdentifierAtom(tree.(*gen.IdentifierAtomContext))
	case *gen.IdentifierContext:
		return v.VisitIdentifier(tree.(*gen.IdentifierContext))
	case *gen.BinaryMultiplicativeExpressionContext:
		return v.VisitBinaryMultiplicativeExpression(tree.(*gen.BinaryMultiplicativeExpressionContext))
	case *gen.BinaryAdditiveExpressionContext:
		return v.VisitBinaryAdditiveExpression(tree.(*gen.BinaryAdditiveExpressionContext))
	case *gen.SubExpressionContext:
		return v.VisitSubExpression(tree.(*gen.SubExpressionContext))
	case *gen.BinaryLogicExpressionContext:
		return v.VisitBinaryLogicExpression(tree.(*gen.BinaryLogicExpressionContext))
	case *gen.BinaryComparisonExpressionContext:
		return v.VisitBinaryComparisonExpression(tree.(*gen.BinaryComparisonExpressionContext))
	case *gen.LikeExpressionContext:
		return v.VisitLikeExpression(tree.(*gen.LikeExpressionContext))
	case *gen.FunctionInvocationExpressionContext:
		return v.VisitFunctionInvocationExpression(tree.(*gen.FunctionInvocationExpressionContext))
	}
	return nil
}

func (v *expressionVisitor) VisitChildren(node antlr.RuleNode) interface{} {
	return v.Visit(node.GetChild(0).(antlr.ParseTree))
}

func (v *expressionVisitor) VisitTerminal(node antlr.TerminalNode) interface{} {
	// We never visit terminal nodes
	return nil
}

func (v *expressionVisitor) VisitErrorNode(node antlr.ErrorNode) interface{} {
	// We already collect errors using the error listener
	return nil
}

// gen.CESQLParserVisitor implementation

func (v *expressionVisitor) VisitInExpression(ctx *gen.InExpressionContext) interface{} {
	leftExpression := v.Visit(ctx.Expression()).(cesql.Expression)

	var setExpression []cesql.Expression

	for _, expr := range ctx.SetExpression().(*gen.SetExpressionContext).AllExpression() {
		setExpression = append(setExpression, v.Visit(expr).(cesql.Expression))
	}

	if ctx.NOT() != nil {
		return expression.NewNotExpression(expression.NewInExpression(leftExpression, setExpression))
	}

	return expression.NewInExpression(leftExpression, setExpression)
}

func (v *expressionVisitor) VisitBinaryComparisonExpression(ctx *gen.BinaryComparisonExpressionContext) interface{} {
	if ctx.LESS() != nil {
		return expression.NewLessExpression(
			v.Visit(ctx.Expression(0)).(cesql.Expression),
			v.Visit(ctx.Expression(1)).(cesql.Expression),
		)
	} else if ctx.LESS_OR_EQUAL() != nil {
		return expression.NewLessOrEqualExpression(
			v.Visit(ctx.Expression(0)).(cesql.Expression),
			v.Visit(ctx.Expression(1)).(cesql.Expression),
		)
	} else if ctx.GREATER() != nil {
		return expression.NewGreaterExpression(
			v.Visit(ctx.Expression(0)).(cesql.Expression),
			v.Visit(ctx.Expression(1)).(cesql.Expression),
		)
	} else if ctx.GREATER_OR_EQUAL() != nil {
		return expression.NewGreaterOrEqualExpression(
			v.Visit(ctx.Expression(0)).(cesql.Expression),
			v.Visit(ctx.Expression(1)).(cesql.Expression),
		)
	} else if ctx.EQUAL() != nil {
		return expression.NewEqualExpression(
			v.Visit(ctx.Expression(0)).(cesql.Expression),
			v.Visit(ctx.Expression(1)).(cesql.Expression),
		)
	} else {
		return expression.NewNotEqualExpression(
			v.Visit(ctx.Expression(0)).(cesql.Expression),
			v.Visit(ctx.Expression(1)).(cesql.Expression),
		)
	}
}

func (v *expressionVisitor) VisitExistsExpression(ctx *gen.ExistsExpressionContext) interface{} {
	return expression.NewExistsExpression(strings.ToLower(ctx.Identifier().GetText()))
}

func (v *expressionVisitor) VisitBinaryLogicExpression(ctx *gen.BinaryLogicExpressionContext) interface{} {
	if ctx.AND() != nil {
		return expression.NewAndExpression(
			v.Visit(ctx.Expression(0)).(cesql.Expression),
			v.Visit(ctx.Expression(1)).(cesql.Expression),
		)
	} else if ctx.OR() != nil {
		return expression.NewOrExpression(
			v.Visit(ctx.Expression(0)).(cesql.Expression),
			v.Visit(ctx.Expression(1)).(cesql.Expression),
		)
	} else {
		return expression.NewXorExpression(
			v.Visit(ctx.Expression(0)).(cesql.Expression),
			v.Visit(ctx.Expression(1)).(cesql.Expression),
		)
	}
}

func (v *expressionVisitor) VisitLikeExpression(ctx *gen.LikeExpressionContext) interface{} {
	patternContext := ctx.StringLiteral().(*gen.StringLiteralContext)

	var pattern string
	if patternContext.DQUOTED_STRING_LITERAL() != nil {
		// Parse double quoted string
		pattern = dQuotedStringToString(patternContext.DQUOTED_STRING_LITERAL().GetText())
	} else {
		// Parse single quoted string
		pattern = sQuotedStringToString(patternContext.SQUOTED_STRING_LITERAL().GetText())
	}

	likeExpression, err := expression.NewLikeExpression(v.Visit(ctx.Expression()).(cesql.Expression), pattern)
	if err != nil {
		v.parsingErrors = append(v.parsingErrors, err)
		return noopExpression{}
	}

	if ctx.NOT() != nil {
		return expression.NewNotExpression(likeExpression)
	}

	return likeExpression
}

func (v *expressionVisitor) VisitFunctionInvocationExpression(ctx *gen.FunctionInvocationExpressionContext) interface{} {
	paramsCtx := ctx.FunctionParameterList().(*gen.FunctionParameterListContext)

	name := ctx.FunctionIdentifier().GetText()

	var args []cesql.Expression
	for _, expr := range paramsCtx.AllExpression() {
		args = append(args, v.Visit(expr).(cesql.Expression))
	}

	return expression.NewFunctionInvocationExpression(strings.ToUpper(name), args)
}

func (v *expressionVisitor) VisitBinaryMultiplicativeExpression(ctx *gen.BinaryMultiplicativeExpressionContext) interface{} {
	if ctx.STAR() != nil {
		return expression.NewMultiplicationExpression(
			v.Visit(ctx.Expression(0)).(cesql.Expression),
			v.Visit(ctx.Expression(1)).(cesql.Expression),
		)
	} else if ctx.MODULE() != nil {
		return expression.NewModuleExpression(
			v.Visit(ctx.Expression(0)).(cesql.Expression),
			v.Visit(ctx.Expression(1)).(cesql.Expression),
		)
	} else {
		return expression.NewDivisionExpression(
			v.Visit(ctx.Expression(0)).(cesql.Expression),
			v.Visit(ctx.Expression(1)).(cesql.Expression),
		)
	}
}

func (v *expressionVisitor) VisitUnaryLogicExpression(ctx *gen.UnaryLogicExpressionContext) interface{} {
	return expression.NewNotExpression(
		v.Visit(ctx.Expression()).(cesql.Expression),
	)
}

func (v *expressionVisitor) VisitUnaryNumericExpression(ctx *gen.UnaryNumericExpressionContext) interface{} {
	return expression.NewNegateExpression(
		v.Visit(ctx.Expression()).(cesql.Expression),
	)
}

func (v *expressionVisitor) VisitSubExpression(ctx *gen.SubExpressionContext) interface{} {
	return v.Visit(ctx.Expression())
}

func (v *expressionVisitor) VisitBinaryAdditiveExpression(ctx *gen.BinaryAdditiveExpressionContext) interface{} {
	if ctx.PLUS() != nil {
		return expression.NewSumExpression(
			v.Visit(ctx.Expression(0)).(cesql.Expression),
			v.Visit(ctx.Expression(1)).(cesql.Expression),
		)
	} else {
		return expression.NewDifferenceExpression(
			v.Visit(ctx.Expression(0)).(cesql.Expression),
			v.Visit(ctx.Expression(1)).(cesql.Expression),
		)
	}
}

func (v *expressionVisitor) VisitIdentifier(ctx *gen.IdentifierContext) interface{} {
	return expression.NewIdentifierExpression(strings.ToLower(ctx.GetText()))
}

func (v *expressionVisitor) VisitBooleanLiteral(ctx *gen.BooleanLiteralContext) interface{} {
	return expression.NewLiteralExpression(ctx.TRUE() != nil)
}

func (v *expressionVisitor) VisitStringLiteral(ctx *gen.StringLiteralContext) interface{} {
	var str string
	if ctx.DQUOTED_STRING_LITERAL() != nil {
		// Parse double quoted string
		str = dQuotedStringToString(ctx.DQUOTED_STRING_LITERAL().GetText())
	} else {
		// Parse single quoted string
		str = sQuotedStringToString(ctx.SQUOTED_STRING_LITERAL().GetText())
	}

	return expression.NewLiteralExpression(str)
}

func (v *expressionVisitor) VisitIntegerLiteral(ctx *gen.IntegerLiteralContext) interface{} {
	val, err := strconv.Atoi(ctx.GetText())
	if err != nil {
		v.parsingErrors = append(v.parsingErrors, err)
	}
	return expression.NewLiteralExpression(int32(val))
}

// gen.CESQLParserVisitor implementation - noop methods

func (v *expressionVisitor) VisitCesql(ctx *gen.CesqlContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *expressionVisitor) VisitAtomExpression(ctx *gen.AtomExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *expressionVisitor) VisitBooleanAtom(ctx *gen.BooleanAtomContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *expressionVisitor) VisitIntegerAtom(ctx *gen.IntegerAtomContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *expressionVisitor) VisitStringAtom(ctx *gen.StringAtomContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *expressionVisitor) VisitIdentifierAtom(ctx *gen.IdentifierAtomContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *expressionVisitor) VisitSetExpression(ctx *gen.SetExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *expressionVisitor) VisitFunctionIdentifier(ctx *gen.FunctionIdentifierContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *expressionVisitor) VisitFunctionParameterList(ctx *gen.FunctionParameterListContext) interface{} {
	return v.VisitChildren(ctx)
}

// noop expression. This is necessary to continue to walk through the tree, even if there's a failure in the parsing

type noopExpression struct{}

func (n noopExpression) Evaluate(cloudevents.Event) (interface{}, error) {
	return 0, nil
}

// Utilities

func dQuotedStringToString(str string) string {
	str = str[1 : len(str)-1]
	return strings.ReplaceAll(str, "\\\"", "\"")
}

func sQuotedStringToString(str string) string {
	str = str[1 : len(str)-1]
	return strings.ReplaceAll(str, "\\'", "'")
}
