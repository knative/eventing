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
)

// computes the lineage from the given vertex with the given input eventtype
func (v *Vertex) Lineage(et *eventingv1beta3.EventType, tfc TransformFunctionContext) *Vertex {
	toExplore := v.OutEdges()
	v.Visit()
	res := v.NewWithSameRef()
	for i := 0; i < len(toExplore); i++ {
		edge := toExplore[i]
		if edge.To().Visited() {
			res.AddEdge(edge.To().NewWithSameRef(), edge.Reference(), NoTransform{}, edge.isDLS)
			continue
		}

		// transform -> nil implies that the path can't be traversed with the current transform and/or context
		if et, tfc := edge.Transform(et, tfc); et != nil {
			res.AddEdge(edge.To().Lineage(et.DeepCopy(), tfc.DeepCopy()), edge.Reference(), NoTransform{}, edge.isDLS)
		}
	}
	// the narrowed eventtype and/or transform function context could be different from a different path, so we have to unvisit before exploring another path
	v.Unvisit()
	return res
}

func MakeEmptyEventType() *eventingv1beta3.EventType {
	return &eventingv1beta3.EventType{}
}
