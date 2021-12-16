/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package expression

import (
	"regexp"
	"strings"

	cesql "github.com/cloudevents/sdk-go/sql/v2"
	"github.com/cloudevents/sdk-go/sql/v2/utils"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type likeExpression struct {
	baseUnaryExpression
	pattern *regexp.Regexp
}

func (l likeExpression) Evaluate(event cloudevents.Event) (interface{}, error) {
	val, err := l.child.Evaluate(event)
	if err != nil {
		return nil, err
	}

	val, err = utils.Cast(val, cesql.StringType)
	if err != nil {
		return nil, err
	}

	return l.pattern.MatchString(val.(string)), nil
}

func NewLikeExpression(child cesql.Expression, pattern string) (cesql.Expression, error) {
	// Converting to regex is not the most performant impl, but it works
	p, err := convertLikePatternToRegex(pattern)
	if err != nil {
		return nil, err
	}

	return likeExpression{
		baseUnaryExpression: baseUnaryExpression{
			child: child,
		},
		pattern: p,
	}, nil
}

func convertLikePatternToRegex(pattern string) (*regexp.Regexp, error) {
	var chunks []string
	chunks = append(chunks, "^")

	var chunk strings.Builder

	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '\\' && i < len(pattern)-1 {
			if pattern[i+1] == '%' {
				// \% case
				chunk.WriteRune('%')
				chunks = append(chunks, "\\Q"+chunk.String()+"\\E")
				chunk.Reset()
				i++
				continue
			} else if pattern[i+1] == '_' {
				// \_ case
				chunk.WriteRune('_')
				chunks = append(chunks, "\\Q"+chunk.String()+"\\E")
				chunk.Reset()
				i++
				continue
			}
		} else if pattern[i] == '_' {
			// replace with .
			chunks = append(chunks, "\\Q"+chunk.String()+"\\E")
			chunk.Reset()
			chunks = append(chunks, ".")
		} else if pattern[i] == '%' {
			// replace with .*
			chunks = append(chunks, "\\Q"+chunk.String()+"\\E")
			chunk.Reset()
			chunks = append(chunks, ".*")
		} else {
			chunk.WriteByte(pattern[i])
		}
	}

	if chunk.Len() != 0 {
		chunks = append(chunks, "\\Q"+chunk.String()+"\\E")
	}

	chunks = append(chunks, "$")

	return regexp.Compile(strings.Join(chunks, ""))
}
