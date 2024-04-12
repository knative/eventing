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
	edges    map[comparableDestination][]*Edge // more than one edge may have the same reference (for example in the case where there is a DLS)
}

type Vertex struct {
	parent   *Graph
	self     *duckv1.Destination
	inEdges  []*Edge
	outEdges []*Edge
	visited  bool
}

type Vertices []*Vertex

type Edge struct {
	transform Transform
	self      *duckv1.Destination
	from      *Vertex
	to        *Vertex
	isDLS     bool
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
type Transform interface {
	Apply(et *eventingv1beta3.EventType, tfc TransformFunctionContext) (*eventingv1beta3.EventType, TransformFunctionContext)
	Name() string
}

// TODO(cali0707): flesh this out more, know we need it, not sure what needs to be in it yet
type TransformFunctionContext struct{}

func (t TransformFunctionContext) DeepCopy() TransformFunctionContext { return t } // TODO(cali0707) implement this once we have fleshed out the transform function context struct

func NewGraph() *Graph {
	return &Graph{
		vertices: make(map[comparableDestination]*Vertex),
		edges:    map[comparableDestination][]*Edge{},
	}
}

func (g *Graph) Vertices() Vertices {
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

// Sources returns all of the sources vertices in a graph
// A source vertex is defined as a graph with an in degree of 0 and an out degree >= 1
// This means that there are only outward edges from the vertex, and no inward edges
func (g *Graph) Sources() Vertices {
	sources := Vertices{}
	for _, v := range g.vertices {
		if v.InDegree() == 0 && v.OutDegree() >= 1 {
			sources = append(sources, v)
		}
	}
	return sources
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

func (v *Vertex) AddEdge(to *Vertex, edgeRef *duckv1.Destination, transform Transform, isDLS bool) {
	edge := &Edge{from: v, to: to, transform: transform, self: edgeRef, isDLS: isDLS}
	v.outEdges = append(v.outEdges, edge)
	to.inEdges = append(to.inEdges, edge)

	if v.parent == nil {
		return
	}

	if _, ok := v.parent.edges[makeComparableDestination(edgeRef)]; !ok {
		v.parent.edges[makeComparableDestination(edgeRef)] = []*Edge{}
	}

	v.parent.edges[makeComparableDestination(edgeRef)] = append(v.parent.edges[makeComparableDestination(edgeRef)], edge)
}

func (g *Graph) GetPrimaryOutEdgeWithRef(edgeRef *duckv1.KReference) *Edge {
	if edges, ok := g.edges[makeComparableDestination(&duckv1.Destination{Ref: edgeRef})]; ok {
		for _, e := range edges {
			if !e.isDLS {
				return e
			}
		}
	}

	return nil
}

func (e *Edge) Transform(et *eventingv1beta3.EventType, tfc TransformFunctionContext) (*eventingv1beta3.EventType, TransformFunctionContext) {
	if et == nil {
		return nil, tfc
	}

	return e.transform.Apply(et, tfc)
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
