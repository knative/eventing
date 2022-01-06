/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

// Code generated from CESQLParser.g4 by ANTLR 4.9. DO NOT EDIT.

package gen // CESQLParser
import "github.com/antlr/antlr4/runtime/Go/antlr"

type BaseCESQLParserVisitor struct {
	*antlr.BaseParseTreeVisitor
}

func (v *BaseCESQLParserVisitor) VisitCesql(ctx *CesqlContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCESQLParserVisitor) VisitInExpression(ctx *InExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCESQLParserVisitor) VisitBinaryComparisonExpression(ctx *BinaryComparisonExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCESQLParserVisitor) VisitAtomExpression(ctx *AtomExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCESQLParserVisitor) VisitExistsExpression(ctx *ExistsExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCESQLParserVisitor) VisitBinaryLogicExpression(ctx *BinaryLogicExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCESQLParserVisitor) VisitLikeExpression(ctx *LikeExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCESQLParserVisitor) VisitFunctionInvocationExpression(ctx *FunctionInvocationExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCESQLParserVisitor) VisitBinaryMultiplicativeExpression(ctx *BinaryMultiplicativeExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCESQLParserVisitor) VisitUnaryLogicExpression(ctx *UnaryLogicExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCESQLParserVisitor) VisitUnaryNumericExpression(ctx *UnaryNumericExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCESQLParserVisitor) VisitSubExpression(ctx *SubExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCESQLParserVisitor) VisitBinaryAdditiveExpression(ctx *BinaryAdditiveExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCESQLParserVisitor) VisitBooleanAtom(ctx *BooleanAtomContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCESQLParserVisitor) VisitIntegerAtom(ctx *IntegerAtomContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCESQLParserVisitor) VisitStringAtom(ctx *StringAtomContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCESQLParserVisitor) VisitIdentifierAtom(ctx *IdentifierAtomContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCESQLParserVisitor) VisitIdentifier(ctx *IdentifierContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCESQLParserVisitor) VisitFunctionIdentifier(ctx *FunctionIdentifierContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCESQLParserVisitor) VisitBooleanLiteral(ctx *BooleanLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCESQLParserVisitor) VisitStringLiteral(ctx *StringLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCESQLParserVisitor) VisitIntegerLiteral(ctx *IntegerLiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCESQLParserVisitor) VisitFunctionParameterList(ctx *FunctionParameterListContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseCESQLParserVisitor) VisitSetExpression(ctx *SetExpressionContext) interface{} {
	return v.VisitChildren(ctx)
}
