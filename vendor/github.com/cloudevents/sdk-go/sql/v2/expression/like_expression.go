/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package expression

import (
	cesql "github.com/cloudevents/sdk-go/sql/v2"
	"github.com/cloudevents/sdk-go/sql/v2/utils"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type likeExpression struct {
	baseUnaryExpression
	pattern string
}

func (l likeExpression) Evaluate(event cloudevents.Event) (interface{}, error) {
	val, err := l.child.Evaluate(event)
	if err != nil {
		return false, err
	}

	val, err = utils.Cast(val, cesql.StringType)
	if err != nil {
		return false, err
	}

	return matchString(val.(string), l.pattern), nil

}

func NewLikeExpression(child cesql.Expression, pattern string) (cesql.Expression, error) {
	return likeExpression{
		baseUnaryExpression: baseUnaryExpression{
			child: child,
		},
		pattern: pattern,
	}, nil
}

func matchString(text, pattern string) bool {
	textLen := len(text)
	patternLen := len(pattern)
	textIdx := 0
	patternIdx := 0
	lastWildcardIdx := -1
	lastMatchIdx := 0

	if patternLen == 0 {
		return patternLen == textLen
	}

	for textIdx < textLen {
		if patternIdx < patternLen-1 && pattern[patternIdx] == '\\' &&
			((pattern[patternIdx+1] == '_' || pattern[patternIdx+1] == '%') &&
				pattern[patternIdx+1] == text[textIdx]) {
			// handle escaped characters -> pattern needs to increment two places here
			patternIdx += 2
			textIdx += 1
		} else if patternIdx < patternLen && (pattern[patternIdx] == '_' || pattern[patternIdx] == text[textIdx]) {
			// handle non escaped characters
			textIdx += 1
			patternIdx += 1
		} else if patternIdx < patternLen && pattern[patternIdx] == '%' {
			// handle wildcard characters
			lastWildcardIdx = patternIdx
			lastMatchIdx = textIdx
			patternIdx += 1
		} else if lastWildcardIdx != -1 {
			// greedy match didn't work, try again from the last known match
			patternIdx = lastWildcardIdx + 1
			lastMatchIdx += 1
			textIdx = lastMatchIdx
		} else {
			return false
		}
	}

	// consume remaining pattern characters as long as they are wildcards
	for patternIdx < patternLen {
		if pattern[patternIdx] != '%' {
			return false
		}

		patternIdx += 1
	}

	return true
}
