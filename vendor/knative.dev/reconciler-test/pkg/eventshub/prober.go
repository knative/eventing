/*
Copyright 2021 The Knative Authors

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

package eventshub

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	conformanceevent "github.com/cloudevents/conformance/pkg/event"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/resources/svc"
)

func NewProber() *EventProber {
	return &EventProber{
		shortNameToName: make(map[string]string),
	}
}

type EventProber struct {
	target          target
	shortNameToName map[string]string
	ids             []string
	senderOptions   []EventsHubOption
	receiverOptions []EventsHubOption
}

type target struct {
	// Need [GVR + Name] OR [URI]
	gvr  schema.GroupVersionResource
	name string
	uri  string
}

type EventInfoCombined struct {
	Sent     EventInfo
	Response EventInfo
}

// SetTargetResource configures the senders target as a GVR and name, used when sender is installed.
func (p *EventProber) SetTargetResource(targetGVR schema.GroupVersionResource, targetName string) {
	p.target = target{
		gvr:  targetGVR,
		name: targetName,
	}
}

// SetTargetKRef configures the senders target as KRef, used when sender is installed.
// Note: namespace is ignored.
func (p *EventProber) SetTargetKRef(ref *duckv1.KReference) error {
	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		return err
	}
	gvk := schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    ref.Kind,
	}
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	p.target = target{
		gvr:  gvr,
		name: ref.Name,
	}
	return nil
}

// SetTargetURI configures the senders target as a URI, used when sender is installed.
func (p *EventProber) SetTargetURI(targetURI string) {
	p.target = target{
		uri: targetURI,
	}
}

// ReceiversRejectFirstN adds DropFirstN to the default config for new receivers.
func (p *EventProber) ReceiversRejectFirstN(n uint) {
	p.receiverOptions = append(p.receiverOptions, DropFirstN(n))
}

// ReceiversRejectResponseCode adds DropEventsResponseCode to the default config for new receivers.
func (p *EventProber) ReceiversRejectResponseCode(code int) {
	p.receiverOptions = append(p.receiverOptions, DropEventsResponseCode(code))
}

// ReceiversHaveResponseDelay adds ResponseWaitTime to the default config for
// new receivers.
func (p *EventProber) ReceiversHaveResponseDelay(delay time.Duration) {
	p.receiverOptions = append(p.receiverOptions, ResponseWaitTime(delay))
}

// ReceiverInstall installs an eventshub receiver into the test env.
func (p *EventProber) ReceiverInstall(prefix string, opts ...EventsHubOption) feature.StepFn {
	name := feature.MakeRandomK8sName(prefix)
	p.shortNameToName[prefix] = name
	opts = append(p.receiverOptions, opts...)
	opts = append(opts, StartReceiver)
	return Install(name, opts...)
}

// SenderInstall installs an eventshub sender resource into the test env.
func (p *EventProber) SenderInstall(prefix string, opts ...EventsHubOption) feature.StepFn {
	name := feature.MakeRandomK8sName(prefix)
	p.shortNameToName[prefix] = name
	return func(ctx context.Context, t feature.T) {
		opts := append(opts, p.senderOptions...)
		if len(p.target.uri) > 0 {
			opts = append(opts, StartSenderURL(p.target.uri))
		} else if !p.target.gvr.Empty() {
			opts = append(opts, StartSenderToResource(p.target.gvr, p.target.name))
		} else {
			t.Fatal("no target is configured for event loop")
		}
		// Install into the env.
		Install(name, opts...)(ctx, t)
	}
}

// SenderDone will poll until the sender sends all expected events.
func (p *EventProber) SenderDone(prefix string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		interval, timeout := environment.PollTimingsFromContext(ctx)
		var events []EventInfoCombined
		err := wait.PollImmediate(interval, timeout, func() (bool, error) {
			events = p.SentBy(ctx, prefix)
			t.Log(p.shortNameToName[prefix], "has sent", len(events))
			if len(events) == len(p.ids) {
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			t.Fatalf("timeout while waiting for sender to deliver all expected events [prefix: %s]: %v\n\n%s\n",
				prefix,
				err,
				stringify("sent", events),
			)
		}
	}
}

// ReceiverDone will poll until the receiver has received all expected events.
func (p *EventProber) ReceiverDone(from, to string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		interval, timeout := environment.PollTimingsFromContext(ctx)
		var (
			sent     []EventInfoCombined
			received []EventInfo
			rejected []EventInfo
		)
		err := wait.PollImmediate(interval, timeout, func() (bool, error) {
			sent = p.SentBy(ctx, from)
			t.Log(p.shortNameToName[from], "has sent", len(sent))

			received = p.ReceivedBy(ctx, to)
			t.Log(p.shortNameToName[to], "has received", len(received))

			rejected = p.RejectedBy(ctx, to)
			t.Log(p.shortNameToName[to], "has rejected", len(rejected))

			if len(sent) == len(received)+len(rejected) {
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			t.Fatalf("timeout while waiting for receiver to receive all expected events [from: %s, to: %s]: %v\n%s\n%s\n%s\n",
				from,
				to,
				err,
				stringify("sent", sent),
				stringify("received", combine(received)),
				stringify("rejected", combine(rejected)),
			)
		}
	}
}

// CorrelateSent takes in an array of mixed Sent / Response events (matched with sentEventMatcher for example)
// and correlates them based on the sequence into a pair.
func CorrelateSent(origin string, in []EventInfo) []EventInfoCombined {
	var out []EventInfoCombined
	// not too many events, this will suffice...
	for i, e := range in {
		if e.Origin == origin && e.Kind == EventSent {
			looking := e.Sequence
			for j := i + 1; j <= len(in)-1; j++ {
				if in[j].Kind == EventResponse && in[j].Sequence == looking {
					out = append(out, EventInfoCombined{Sent: e, Response: in[j]})
				}
			}
		}
	}
	return out
}

// SentBy returns events sent by the named sender.
func (p *EventProber) SentBy(ctx context.Context, prefix string) []EventInfoCombined {
	name := p.shortNameToName[prefix]
	store := StoreFromContext(ctx, name)

	return CorrelateSent(name, store.Collected())
}

// ReceivedBy returns events received by the named receiver.
func (p *EventProber) ReceivedBy(ctx context.Context, prefix string) []EventInfo {
	name := p.shortNameToName[prefix]
	store := StoreFromContext(ctx, name)

	events, _, _, _ := store.Find(func(info EventInfo) error {
		if info.Observer == name && info.Kind == EventReceived {
			return nil
		}
		return errors.New("not a match")
	})

	return events
}

// RejectedBy returns events rejected by the named receiver.
func (p *EventProber) RejectedBy(ctx context.Context, prefix string) []EventInfo {
	name := p.shortNameToName[prefix]
	store := StoreFromContext(ctx, name)

	events, _, _, _ := store.Find(func(info EventInfo) error {
		if info.Observer == name && info.Kind == EventRejected {
			return nil
		}
		return errors.New("not a match")
	})

	return events
}

// ReceivedOrRejectedBy returns events received or rejected by the named receiver.
func (p *EventProber) ReceivedOrRejectedBy(ctx context.Context, prefix string) []EventInfo {
	name := p.shortNameToName[prefix]
	store := StoreFromContext(ctx, name)

	events, _, _, _ := store.Find(func(info EventInfo) error {
		if info.Observer == name && (info.Kind == EventReceived || info.Kind == EventRejected) {
			return nil
		}
		return errors.New("not a match")
	})

	return events
}

// ExpectYAMLEvents registered expected events into the prober.
func (p *EventProber) ExpectYAMLEvents(path string) error {
	events, err := conformanceevent.FromYaml(path, true)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return fmt.Errorf("failed to load events from %q", path)
	}
	for _, event := range events {
		p.ids = append(p.ids, event.Attributes.ID)
	}
	return nil
}

// ExpectEvents registers event IDs into the prober.
func (p *EventProber) ExpectEvents(ids []string) {
	p.ids = append(p.ids, ids...)
}

// SenderEventsFromURI configures a sender to send a url/yaml based events.
func (p *EventProber) SenderEventsFromURI(uri string) {
	p.senderOptions = append(p.senderOptions, InputYAML(uri))
}

// SenderEventsFromSVC configures a sender to send a yaml based events fetched
// a service in the testing environment. Namespace of the svc will come from
// env.Namespace(), based on context from the StepFn.
func (p *EventProber) SenderEventsFromSVC(svcName, path string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		u, err := svc.Address(ctx, svcName)
		if err != nil {
			t.Error(err)
		}
		u.Path = path
		p.senderOptions = append(p.senderOptions, InputYAML(u.String()))
	}
}

// SenderFullEvents creates `count` cloudevents.FullEvent events with new IDs into a
// sender and registers them for the prober.
// Warning: only call once.
func (p *EventProber) SenderFullEvents(count int) {
	event := cetest.FullEvent()
	if count == 1 {
		id := uuid.New().String()
		event.SetID(id)
		p.ids = []string{id}
		p.senderOptions = append(p.senderOptions, InputEvent(event))
	} else {
		p.senderOptions = append(p.senderOptions,
			InputEvent(event), SendMultipleEvents(count, 10*time.Millisecond), EnableIncrementalId) // TODO: make configurable.
		p.ids = []string{}
		for i := 1; i <= count; i++ {
			p.ids = append(p.ids, strconv.Itoa(i))
		}
	}
}

// SenderMinEvents creates `count` cloudevents.MinEvent events with new IDs into a
// sender and registers them for the prober.
// Warning: only call once.
func (p *EventProber) SenderMinEvents(count int) {
	event := cetest.MinEvent()
	if count == 1 {
		id := uuid.New().String()
		event.SetID(id)
		p.ids = []string{id}
		p.senderOptions = append(p.senderOptions, InputEvent(event))
	} else {
		p.senderOptions = append(p.senderOptions, InputEvent(event), SendMultipleEvents(count, 10*time.Millisecond)) // TODO: make configurable.
		p.ids = []string{}
		for i := 1; i <= count; i++ {
			p.ids = append(p.ids, strconv.Itoa(i))
		}
	}
}

// AsKReference returns the short-named component as a KReference.
func (p *EventProber) AsKReference(prefix string) *duckv1.KReference {
	return svc.AsKReference(p.shortNameToName[prefix])
}

// AssertSentAll tests that `fromPrefix` sent all known events known to the prober.
func (p *EventProber) AssertSentAll(fromPrefix string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		events := p.SentBy(ctx, fromPrefix)
		if len(p.ids) != len(events) {
			t.Errorf("expected %q to have sent %d events, actually sent %d",
				fromPrefix, len(p.ids), len(events))
		}
		for _, id := range p.ids {
			found := false
			for _, event := range events {
				if id == event.Sent.SentId {
					found = true
					if event.Response.StatusCode < 200 || event.Response.StatusCode > 299 {
						t.Errorf("Failed to send event id=%s, %s", id, event.Response.String())
					}
					break
				}
			}
			if !found {
				t.Errorf("Failed to send event id=%s", id)
			}
		}
	}
}

// AssertReceivedAll tests that all events sent by `fromPrefix` were received by `toPrefix`.
func (p *EventProber) AssertReceivedAll(fromPrefix, toPrefix string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		sent := p.SentBy(ctx, fromPrefix)
		ids := make([]string, len(sent))
		for i, s := range sent {
			ids[i] = s.Sent.SentId
		}

		events := p.ReceivedBy(ctx, toPrefix)
		if len(ids) != len(events) {
			t.Errorf("expected %q to have received %d events, actually received %d",
				toPrefix, len(ids), len(events))
		}
		for _, id := range ids {
			found := false
			for _, event := range events {
				if event.Event != nil && id == event.Event.ID() {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Failed to receive event id=%s", id)
			}
		}
	}
}

// AssertReceivedAll tests that all events sent by `fromPrefix` were received by `toPrefix`.
func (p *EventProber) AssertReceivedOrRejectedAll(fromPrefix, toPrefix string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		sent := p.SentBy(ctx, fromPrefix)
		ids := make([]string, len(sent))
		for i, s := range sent {
			ids[i] = s.Sent.SentId
		}

		events := p.ReceivedOrRejectedBy(ctx, toPrefix)

		if len(ids) != len(events) {
			t.Errorf("expected %q to have received %d events, actually received %d",
				fromPrefix, len(ids), len(events))
		}
		for _, id := range ids {
			found := false
			for _, event := range events {
				if event.Event != nil && id == event.Event.ID() {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Failed to receive event id=%s", id)
			}
		}
	}
}

func stringify(title string, events []EventInfoCombined) string {
	errorMsg := title + "\n"
	for _, e := range events {
		errorMsg += "Sent: " + e.Sent.String() + "\nResponse: " + e.Response.String() + "\n\n"
	}
	return errorMsg
}

func combine(ei []EventInfo) []EventInfoCombined {
	var c []EventInfoCombined
	for _, e := range ei {
		c = append(c, EventInfoCombined{Sent: e})
	}
	return c
}

// AssertReceivedNone tests that no events sent by `fromPrefix` were received by `toPrefix`.
func (p *EventProber) AssertReceivedNone(fromPrefix, toPrefix string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {

		events := p.ReceivedBy(ctx, toPrefix)
		if len(events) > 0 {
			t.Errorf("expected %q to not have received any events from %s, actually received %d",
				toPrefix, fromPrefix, len(events))
		}
	}
}
