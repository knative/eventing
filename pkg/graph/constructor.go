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
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func (g *Graph) AddBroker(broker eventingv1.Broker) {
	ref := &duckv1.KReference{
		Name:       broker.Name,
		Namespace:  broker.Namespace,
		APIVersion: "eventing.knative.dev/v1",
		Kind:       "Broker",
	}
	dest := &duckv1.Destination{Ref: ref}

	// check if this vertex already exists
	v, ok := g.vertices[makeComparableDestination(dest)]
	if !ok {
		v = &Vertex{
			self: dest,
		}
		g.vertices[makeComparableDestination(dest)] = v
	}

	if broker.Spec.Delivery == nil || broker.Spec.Delivery.DeadLetterSink == nil {
		// no DLS, we are done
		return
	}

	// broker has a DLS, we need to add an edge to that
	to, ok := g.vertices[makeComparableDestination(broker.Spec.Delivery.DeadLetterSink)]
	if !ok {
		to = &Vertex{
			self: broker.Spec.Delivery.DeadLetterSink,
		}
		g.vertices[makeComparableDestination(broker.Spec.Delivery.DeadLetterSink)] = to
	}

	v.AddEdge(to, dest, NoTransform)
}
