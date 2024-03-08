/*
Copyright 2024 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package graph

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	eventingv1beta3 "knative.dev/eventing/pkg/apis/eventing/v1beta3"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestSingleVertexLineage(t *testing.T) {
	a := &Vertex{
		self: &duckv1.Destination{
			Ref: &duckv1.KReference{
				Name: "A",
			},
		},
	}
	b := &Vertex{
		self: &duckv1.Destination{
			Ref: &duckv1.KReference{
				Name: "B",
			},
		},
	}
	c := &Vertex{
		self: &duckv1.Destination{
			Ref: &duckv1.KReference{
				Name: "C",
			},
		},
	}
	d := &Vertex{
		self: &duckv1.Destination{
			Ref: &duckv1.KReference{
				Name: "D",
			},
		},
	}
	e := &Vertex{
		self: &duckv1.Destination{
			Ref: &duckv1.KReference{
				Name: "E",
			},
		},
	}

	a.AddEdge(b, nil, NoTransform)
	b.AddEdge(c, nil, func(et *eventingv1beta3.EventType, tfc TransformFunctionContext) (*eventingv1beta3.EventType, TransformFunctionContext) {
		if et.Spec.Type == "" {
			et.Spec.Type = "example.type"
		}

		if et.Spec.Type != "example.type" {
			return nil, tfc
		}

		return et, tfc
	})
	b.AddEdge(d, nil, NoTransform)
	c.AddEdge(d, nil, func(et *eventingv1beta3.EventType, tfc TransformFunctionContext) (*eventingv1beta3.EventType, TransformFunctionContext) {
		if et.Spec.Type == "" {
			et.Spec.Type = "some.other.type"
		}

		if et.Spec.Type != "some.other.type" {
			return nil, tfc
		}

		return et, tfc
	})
	c.AddEdge(e, nil, NoTransform)
	d.AddEdge(e, nil, NoTransform)
	e.AddEdge(c, nil, NoTransform)

	lineageFromA := a.Lineage(MakeEmptyEventType(), TransformFunctionContext{})
	assert.Equal(t, "A", lineageFromA.Reference().Ref.Name)
	assert.Equal(t, 1, lineageFromA.OutDegree())

	lineageFromB := lineageFromA.OutEdges()[0].To()
	assert.Equal(t, "B", lineageFromB.Reference().Ref.Name)
	assert.Equal(t, 2, lineageFromB.OutDegree())

	// two paths, we don't make guarantees on the order the lineage algorithm traverses them so we need to figure out which is which
	var lineageFromC *Vertex
	var lineageFromD *Vertex
	if lineageFromB.OutEdges()[0].To().Reference().Ref.Name == "C" {
		lineageFromC = lineageFromB.OutEdges()[0].To()
	} else if lineageFromB.OutEdges()[1].To().Reference().Ref.Name == "C" {
		lineageFromC = lineageFromB.OutEdges()[1].To()
	}

	if lineageFromB.OutEdges()[0].To().Reference().Ref.Name == "D" {
		lineageFromD = lineageFromB.OutEdges()[0].To()
	} else if lineageFromB.OutEdges()[1].To().Reference().Ref.Name == "D" {
		lineageFromD = lineageFromB.OutEdges()[1].To()
	}

	assert.NotNil(t, lineageFromC)
	assert.NotNil(t, lineageFromD)

	// assert that the path from C goes to E but not D, it should also have the loop back to C but not go further
	assert.Equal(t, 1, lineageFromC.OutDegree())
	assert.Equal(t, "E", lineageFromC.OutEdges()[0].To().Reference().Ref.Name)
	assert.Equal(t, 1, lineageFromC.OutEdges()[0].To().OutDegree())
	assert.Equal(t, "C", lineageFromC.OutEdges()[0].To().OutEdges()[0].To().Reference().Ref.Name)
	assert.Equal(t, 0, lineageFromC.OutEdges()[0].To().OutEdges()[0].To().OutDegree())

	// assert that the path from D goes to E and then C
	assert.Equal(t, 1, lineageFromD.OutDegree())
	assert.Equal(t, "E", lineageFromD.OutEdges()[0].To().Reference().Ref.Name)
	assert.Equal(t, 1, lineageFromD.OutEdges()[0].To().OutDegree())
	assert.Equal(t, "C", lineageFromD.OutEdges()[0].To().OutEdges()[0].To().Reference().Ref.Name)

	// once back to C, we should have two cycles: one back to E, and one to D
	lineageFromCFromD := lineageFromD.OutEdges()[0].To().OutEdges()[0].To()
	assert.Equal(t, 2, lineageFromCFromD.OutDegree())
	expectedVertices := []string{"E", "D"}
	for _, edge := range lineageFromCFromD.OutEdges() {
		assert.Contains(t, expectedVertices, edge.To().Reference().Ref.Name)
		assert.Equal(t, 0, edge.To().OutDegree())

		// remove the vertex from the expected set, so that we know that both expected vertices are there
		idx := slices.Index(expectedVertices, edge.To().Reference().Ref.Name)
		expectedVertices = slices.Delete(expectedVertices, idx, idx+1)
	}
}
