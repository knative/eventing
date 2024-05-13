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
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	eventingv1beta3 "knative.dev/eventing/pkg/apis/eventing/v1beta3"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func ConstructGraph(ctx context.Context, filterFunc func(obj interface{}) bool) (*Graph, error) {
	eventingClient := eventingclient.Get(ctx)

	g := NewGraph()

	brokers, err := eventingClient.EventingV1().Brokers("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, broker := range brokers.Items {
		if filterFunc(broker) {
			g.AddBroker(broker)
		}
	}

	channels, err := eventingClient.MessagingV1().Channels("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, channel := range channels.Items {
		if filterFunc(channel) {
			g.AddChannel(channel)
		}
	}

	apiServerSources, err := eventingClient.SourcesV1().ApiServerSources("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, apiServerSource := range apiServerSources.Items {
		if filterFunc(apiServerSource) {
			g.AddSource(duckv1.Source{
				ObjectMeta: apiServerSource.ObjectMeta,
				TypeMeta:   apiServerSource.TypeMeta,
				Spec:       apiServerSource.Spec.SourceSpec,
				Status:     apiServerSource.Status.SourceStatus,
			})
		}
	}

	containerSources, err := eventingClient.SourcesV1().ContainerSources("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, containerSource := range containerSources.Items {
		if filterFunc(containerSource) {
			g.AddSource(duckv1.Source{
				ObjectMeta: containerSource.ObjectMeta,
				TypeMeta:   containerSource.TypeMeta,
				Spec:       containerSource.Spec.SourceSpec,
				Status:     containerSource.Status.SourceStatus,
			})
		}
	}

	pingSources, err := eventingClient.SourcesV1().PingSources("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, pingSource := range pingSources.Items {
		if filterFunc(pingSource) {
			g.AddSource(duckv1.Source{
				ObjectMeta: pingSource.ObjectMeta,
				TypeMeta:   pingSource.TypeMeta,
				Spec:       pingSource.Spec.SourceSpec,
				Status:     pingSource.Status.SourceStatus,
			})
		}
	}

	sinkBindings, err := eventingClient.SourcesV1().SinkBindings("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, sinkBinding := range sinkBindings.Items {
		if filterFunc(sinkBinding) {
			g.AddSource(duckv1.Source{
				ObjectMeta: sinkBinding.ObjectMeta,
				TypeMeta:   sinkBinding.TypeMeta,
				Spec:       sinkBinding.Spec.SourceSpec,
				Status:     sinkBinding.Status.SourceStatus,
			})
		}
	}

	triggers, err := eventingClient.EventingV1().Triggers("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, trigger := range triggers.Items {
		if filterFunc(trigger) {
			err := g.AddTrigger(trigger)
			if err != nil {
				return nil, err
			}
		}
	}

	subscriptions, err := eventingClient.MessagingV1().Subscriptions("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, subscription := range subscriptions.Items {
		if filterFunc(subscription) {
			err := g.AddSubscription(subscription)
			if err != nil {
				return nil, err
			}
		}
	}

	eventTypes, err := eventingClient.EventingV1beta3().EventTypes("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, eventType := range eventTypes.Items {
		if filterFunc(eventType) {
			err := g.AddEventType(eventType)
			if err != nil {
				return nil, err
			}
		}
	}

	return g, nil
}

func (g *Graph) AddBroker(broker eventingv1.Broker) {
	ref := &duckv1.KReference{
		Name:       broker.Name,
		Namespace:  broker.Namespace,
		APIVersion: "eventing.knative.dev/v1",
		Kind:       "Broker",
	}
	dest := &duckv1.Destination{Ref: ref}

	// check if this vertex already exists
	v := g.getOrCreateVertex(dest)

	if broker.Spec.Delivery == nil || broker.Spec.Delivery.DeadLetterSink == nil {
		// no DLS, we are done
		return
	}

	// broker has a DLS, we need to add an edge to that
	to := g.getOrCreateVertex(broker.Spec.Delivery.DeadLetterSink)

	v.AddEdge(to, dest, NoTransform{}, true)
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

	v := g.getOrCreateVertex(dest)

	if channel.Spec.Delivery == nil || channel.Spec.Delivery.DeadLetterSink == nil {
		// no DLS, we are done
		return
	}

	// channel has a DLS, we need to add an edge to that
	to := g.getOrCreateVertex(channel.Spec.Delivery.DeadLetterSink)

	v.AddEdge(to, dest, NoTransform{}, true)
}

func (g *Graph) AddEventType(et eventingv1beta3.EventType) error {
	ref := &duckv1.KReference{
		Name:       et.Name,
		Namespace:  et.Namespace,
		APIVersion: "eventing.knative.dev/v1beta3",
		Kind:       "EventType",
	}
	dest := &duckv1.Destination{Ref: ref}

	if et.Spec.Reference.Kind == "Subscription" || et.Spec.Reference.Kind == "Trigger" {
		outEdge := g.GetPrimaryOutEdgeWithRef(et.Spec.Reference)
		if outEdge == nil {
			return fmt.Errorf("trigger/subscription must have a primary outward edge, but had none")
		}

		outEdge.To().AddEdge(outEdge.From(), dest, EventTypeTransform{EventType: &et}, false)

		return nil
	}

	from := g.getOrCreateVertex(dest)
	to := g.getOrCreateVertex(&duckv1.Destination{Ref: et.Spec.Reference})

	from.AddEdge(to, dest, EventTypeTransform{EventType: &et}, false)

	return nil
}

func (g *Graph) AddSource(source duckv1.Source) {
	ref := &duckv1.KReference{
		Name:       source.Name,
		Namespace:  source.Namespace,
		APIVersion: source.APIVersion,
		Kind:       source.Kind,
	}
	dest := &duckv1.Destination{Ref: ref}

	v := g.getOrCreateVertex(dest)

	to := g.getOrCreateVertex(&source.Spec.Sink)

	v.AddEdge(to, dest, CloudEventOverridesTransform{Overrides: source.Spec.CloudEventOverrides}, true)
}

func (g *Graph) getOrCreateVertex(dest *duckv1.Destination) *Vertex {
	v, ok := g.vertices[makeComparableDestination(dest)]
	if !ok {
		v = &Vertex{
			self:   dest,
			parent: g,
		}
		g.vertices[makeComparableDestination(dest)] = v
	}

	return v
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

	to := g.getOrCreateVertex(&trigger.Spec.Subscriber)

	//TODO: the transform function should be set according to the trigger filter - there are multiple open issues to address this later
	broker.AddEdge(to, triggerDest, NoTransform{}, false)

	if trigger.Spec.Delivery == nil || trigger.Spec.Delivery.DeadLetterSink == nil {
		return nil
	}

	dls := g.getOrCreateVertex(trigger.Spec.Delivery.DeadLetterSink)

	broker.AddEdge(dls, triggerDest, NoTransform{}, true)

	return nil

}
func (g *Graph) AddSubscription(subscription messagingv1.Subscription) error {
	channelRef := &duckv1.KReference{
		Name:       subscription.Spec.Channel.Name,
		Namespace:  subscription.Namespace,
		APIVersion: subscription.Spec.Channel.APIVersion,
		Kind:       subscription.Spec.Channel.Kind,
	}
	channelDest := &duckv1.Destination{Ref: channelRef}
	channel, ok := g.vertices[makeComparableDestination(channelDest)]

	if !ok {
		return fmt.Errorf("subscription refers to a non existent channel, can't add it to the graph")
	}

	subscriptionRef := &duckv1.KReference{
		Name:       subscription.Name,
		Namespace:  subscription.Namespace,
		APIVersion: subscription.APIVersion,
		Kind:       "Subscription",
	}
	subscriptionDest := &duckv1.Destination{Ref: subscriptionRef}

	to := g.getOrCreateVertex(subscription.Spec.Subscriber)
	channel.AddEdge(to, subscriptionDest, NoTransform{}, false)

	// If the subscription has a reply field set, there should be another Edge struct.
	if subscription.Spec.Reply != nil {
		reply := g.getOrCreateVertex(subscription.Spec.Reply)
		to.AddEdge(reply, subscriptionDest, NoTransform{}, false)
	}

	// If the subscription has the deadLetterSink property set on the delivery field, then another Edge should be constructed.
	if subscription.Spec.Delivery == nil || subscription.Spec.Delivery.DeadLetterSink == nil {
		return nil
	}
	dls := g.getOrCreateVertex(subscription.Spec.Delivery.DeadLetterSink)
	channel.AddEdge(dls, subscriptionDest, NoTransform{}, true)

	return nil

}
