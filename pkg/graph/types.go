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
	eventingv1beta3 "knative.dev/eventing/pkg/apis/eventing/v1beta3"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

type Graph struct {
	vertices map[comparableDestination]*Vertex
}

type Vertex struct {
	self     *duckv1.Destination
	inEdges  []*Edge
	outEdges []*Edge
	visited  bool
}

type Edge struct {
	transform TransformFunction
	self      *duckv1.Destination
	from      *Vertex
	to        *Vertex
}

// comparableDestination is a modified version of duckv1.Destination that is comparable (no pointers).
// It also omits some fields not needed for event lineage.
type comparableDestination struct {
	// Ref points to an Addressable.
	// +optional
	Ref duckv1.KReference `json:"ref,omitempty"`

	// URI can be an absolute URL(non-empty scheme and non-empty host) pointing to the target or a relative URI. Relative URIs will be resolved using the base URI retrieved from Ref.
	// +optional
	URI apis.URL `json:"uri,omitempty"`
}
type TransformFunction func(et *eventingv1beta3.EventType, tfc TransformFunctionContext) (*eventingv1beta3.EventType, TransformFunctionContext)

// TODO(cali0707): flesh this out more, know we need it, not sure what needs to be in it yet
type TransformFunctionContext struct{}

func (t TransformFunctionContext) DeepCopy() TransformFunctionContext { return t } // TODO(cali0707) implement this once we have fleshed out the transform function context struct

func NewGraph() *Graph {
	return &Graph{
		vertices: make(map[comparableDestination]*Vertex),
	}
}

func (g *Graph) Vertices() []*Vertex {
	vertices := make([]*Vertex, len(g.vertices))
	for _, v := range g.vertices {
		vertices = append(vertices, v)
	}
	return vertices
}

func (g *Graph) UnvisitAll() {
	for _, v := range g.vertices {
		v.visited = false
	}
}

func (v *Vertex) InDegree() int {
	return len(v.inEdges)
}

func (v *Vertex) OutDegree() int {
	return len(v.outEdges)
}

func (v *Vertex) Reference() *duckv1.Destination {
	return v.self
}

func (v *Vertex) InEdges() []*Edge {
	return v.inEdges
}

func (v *Vertex) OutEdges() []*Edge {
	return v.outEdges
}

func (v *Vertex) Visit() {
	v.visited = true
}

func (v *Vertex) Unvisit() {
	v.visited = false
}

func (v *Vertex) Visited() bool {
	return v.visited
}

func (v *Vertex) NewWithSameRef() *Vertex {
	return &Vertex{
		self: v.self,
	}
}

func (v *Vertex) AddEdge(to *Vertex, edgeRef *duckv1.Destination, transform TransformFunction) {
	edge := &Edge{from: v, to: to, transform: transform, self: edgeRef}
	v.outEdges = append(v.outEdges, edge)
	to.inEdges = append(to.inEdges, edge)

}

func (e *Edge) Transform(et *eventingv1beta3.EventType, tfc TransformFunctionContext) (*eventingv1beta3.EventType, TransformFunctionContext) {
	if et == nil {
		return nil, tfc
	}

	return e.transform(et, tfc)
}

func (e *Edge) From() *Vertex {
	return e.from
}

func (e *Edge) To() *Vertex {
	return e.to
}

func (e *Edge) Reference() *duckv1.Destination {
	return e.self
}

func NoTransform(et *eventingv1beta3.EventType, tfc TransformFunctionContext) (*eventingv1beta3.EventType, TransformFunctionContext) {
	return et, tfc
}

func makeComparableDestination(dest *duckv1.Destination) comparableDestination {
	res := comparableDestination{}
	if dest.Ref != nil {
		res.Ref = *dest.Ref
	}
	if dest.URI != nil {
		res.URI = *dest.URI
	}
	return res
}
