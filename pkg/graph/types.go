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
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

type Graph struct {
	vertices []*Vertex
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

type TransformFunction func(et *eventingv1beta3.EventType, tfc TransformFunctionContext) (*eventingv1beta3.EventType, TransformFunctionContext)

// TODO(cali0707): flesh this out more, know we need it, not sure what needs to be in it yet
type TransformFunctionContext struct{}

func (t TransformFunctionContext) DeepCopy() TransformFunctionContext { return t } // TODO(cali0707) implement this once we have fleshed out the transform function context struct

func (g *Graph) Vertices() []*Vertex {
	return g.vertices
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
	v.outEdges = append(v.outEdges, &Edge{from: v, to: to, transform: transform, self: edgeRef})

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
