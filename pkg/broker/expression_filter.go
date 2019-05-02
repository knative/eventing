/*
 * Copyright 2019 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package broker

import (
	"fmt"
	"strings"

	"github.com/cloudevents/sdk-go"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	"go.uber.org/zap"
)

const (
	// CELVarKeyContext is the CEL variable key used for CloudEvents event
	// context attributes, both official and extension.
	CELVarKeyContext = "ce"
	// CELVarKeyData is the CEL variable key used for parsed, structured event
	// data.
	CELVarKeyData = "data"
)

func expressionFromAttributes(attrs map[string]string) string {
	clauses := []string{}
	for k, v := range attrs {
		clauses = append(clauses, fmt.Sprintf(`ce[%q] == %q`, k, v))
	}
	return strings.Join(clauses, " && ")
}

func (r *Receiver) filterEventByExpression(expr string, event *cloudevents.Event) (bool, error) {
	prg, parseData, err := getOrCacheProgram(expr, programForExpression)
	if err != nil {
		return false, err
	}

	// Set baseline context attributes. The attributes available may not be
	// exactly the same as the attributes defined in the current version of the
	// CloudEvents spec.
	ce := map[string]interface{}{
		"specversion": event.SpecVersion(),
		"type":        event.Type(),
		"source":      event.Source(),
		"subject":     event.Subject(),
		"id":          event.ID(),
		// TODO Time. This should be a protobuf Timestamp
		"schemaurl":           event.SchemaURL(),
		"datacontenttype":     event.DataContentType(),
		"datamediatype":       event.DataMediaType(),
		"datacontentencoding": event.DataContentEncoding(),
	}

	// TODO stop coercing to v02
	ext := event.Context.AsV02().Extensions
	if ext != nil {
		for k, v := range ext {
			ce[k] = v
		}
	}

	// If the program requires parsing of data, attempt to parse into a map.
	data := make(map[string]interface{})
	if parseData {
		// TODO Does this parse the event data multiple times? We should only
		// parse once per event regardless of Trigger count.
		if err := event.DataAs(&data); err != nil {
			r.logger.Error("Failed to parse event data for CEL filtering", zap.String("id", event.Context.AsV02().ID), zap.Error(err))
		}
	}

	out, _, err := prg.Eval(map[string]interface{}{
		CELVarKeyContext: ce,
		CELVarKeyData:    data,
	})
	if err != nil {
		return false, err
	}

	return out == types.True, nil
}

func programForExpression(expr string) (cel.Program, bool, error) {
	env, err := cel.NewEnv(
		cel.Declarations(
			decls.NewIdent(CELVarKeyContext, decls.Dyn, nil),
			decls.NewIdent(CELVarKeyData, decls.Dyn, nil),
		),
	)
	if err != nil {
		return nil, false, err
	}

	parsedAst, iss := env.Parse(expr)
	if iss != nil && iss.Err() != nil {
		return nil, false, iss.Err()
	}
	checkedAst, iss := env.Check(parsedAst)
	if iss != nil && iss.Err() != nil {
		return nil, false, iss.Err()
	}

	checkedExpr, err := cel.AstToCheckedExpr(checkedAst)
	if err != nil {
		return nil, false, err
	}

	prg, err := env.Program(checkedAst)
	if err != nil {
		return nil, false, err
	}

	// Check if the expression references the data variable. If so, return true
	// for parseData.
	for _, r := range checkedExpr.ReferenceMap {
		if r.Name == "data" {
			return prg, true, nil
		}
	}

	return prg, false, nil
}
