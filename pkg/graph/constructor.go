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
	"fmt"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
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

func (g *Graph) AddChannel(channel messagingv1.Channel) {
	if channel.Kind == "" {
		channel.Kind = "Channel"
	}

	ref := &duckv1.KReference{
		Name:       channel.Name,
		Namespace:  channel.Namespace,
		APIVersion: "messaging.knative.dev/v1",
		Kind:       channel.Kind,
	}
	dest := &duckv1.Destination{Ref: ref}

	v, ok := g.vertices[makeComparableDestination(dest)]
	if !ok {
		v = &Vertex{
			self: dest,
		}
		g.vertices[makeComparableDestination(dest)] = v
	}

	if channel.Spec.Delivery == nil || channel.Spec.Delivery.DeadLetterSink == nil {
		// no DLS, we are done
		return
	}

	// channel has a DLS, we need to add an edge to that
	to, ok := g.vertices[makeComparableDestination(channel.Spec.Delivery.DeadLetterSink)]
	if !ok {
		to = &Vertex{
			self: channel.Spec.Delivery.DeadLetterSink,
		}
		g.vertices[makeComparableDestination(channel.Spec.Delivery.DeadLetterSink)] = to
	}

	v.AddEdge(to, dest, NoTransform)
}

func (g *Graph) AddTrigger(trigger eventingv1.Trigger) error {
	brokerRef := &duckv1.KReference{
		Name:       trigger.Spec.Broker,
		Namespace:  trigger.Namespace,
		APIVersion: "eventing.knative.dev/v1",
		Kind:       "Broker",
	}
	brokerDest := &duckv1.Destination{Ref: brokerRef}
	broker, ok := g.vertices[makeComparableDestination(brokerDest)]
	if !ok {
		return fmt.Errorf("trigger refers to a non existent broker, can't add it to the graph")
	}

	triggerRef := &duckv1.KReference{
		Name:       trigger.Name,
		Namespace:  trigger.Namespace,
		APIVersion: "eventing.knative.dev/v1",
		Kind:       "Trigger",
	}
	triggerDest := &duckv1.Destination{Ref: triggerRef}

	to, ok := g.vertices[makeComparableDestination(&trigger.Spec.Subscriber)]
	if !ok {
		to = &Vertex{
			self: &trigger.Spec.Subscriber,
		}
		g.vertices[makeComparableDestination(&trigger.Spec.Subscriber)] = to
	}

	//TODO: the transform function should be set according to the trigger filter - there are multiple open issues to address this later
	broker.AddEdge(to, triggerDest, NoTransform)

	if trigger.Spec.Delivery == nil || trigger.Spec.Delivery.DeadLetterSink == nil {
		return nil
	}

	dls, ok := g.vertices[makeComparableDestination(trigger.Spec.Delivery.DeadLetterSink)]
	if !ok {
		dls = &Vertex{
			self: trigger.Spec.Delivery.DeadLetterSink,
		}
		g.vertices[makeComparableDestination(trigger.Spec.Delivery.DeadLetterSink)] = dls
	}

	broker.AddEdge(dls, triggerDest, NoTransform)

	return nil

}
