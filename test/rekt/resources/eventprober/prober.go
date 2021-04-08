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

package eventprober

import (
	"context"
	"fmt"

	conformanceevent "github.com/cloudevents/conformance/pkg/event"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/eventing/test/rekt/resources/delivery"
	"knative.dev/eventing/test/rekt/resources/svc"
	"knative.dev/eventing/test/rekt/resources/trigger"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/eventshub"
	. "knative.dev/reconciler-test/pkg/eventshub/assert"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
)

func New(targetGVR schema.GroupVersionResource, targetName, targetURI string) *EventProber {
	return &EventProber{
		target: target{
			gvr:  targetGVR,
			name: targetName,
			uri:  targetURI,
		},
		shortNameToName: make(map[string]string),
	}
}

type EventProber struct {
	target target

	shortNameToName map[string]string

	hasCache      bool
	ids           []string
	senderOptions []eventshub.EventsHubOption
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

func (p *EventProber) RxInstall(prefix string) feature.StepFn {
	name := feature.MakeRandomK8sName(prefix)
	p.shortNameToName[prefix] = name
	return eventshub.Install(name, eventshub.StartReceiver)
}

func (p *EventProber) TxDone(prefix string) feature.StepFn {
	//	name := p.shortNameToName[prefix]
	return func(ctx context.Context, t feature.T) {
		interval, timeout := environment.PollTimingsFromContext(ctx)
		err := wait.PollImmediate(interval, timeout, func() (bool, error) {
			events := p.SentBy(ctx, prefix)
			if len(events) == len(p.ids) {
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			t.Failed()
		}
	}
}

func (p *EventProber) TxInstall(prefix string) feature.StepFn {
	name := feature.MakeRandomK8sName(prefix)
	p.shortNameToName[prefix] = name
	return func(ctx context.Context, t feature.T) {
		var opts []eventshub.EventsHubOption
		if len(p.target.uri) > 0 {
			opts = append(p.senderOptions, eventshub.StartSenderURL(p.target.uri))
		} else if !p.target.gvr.Empty() {
			opts = append(p.senderOptions, eventshub.StartSenderToResource(p.target.gvr, p.target.name))
		} else {
			t.Fatal("no target is configured for event loop")
		}
		// Install into the env.
		eventshub.Install(name, opts...)(ctx, t)
	}
}

// Correlate takes in an array of mixed Sent / Response events (matched with sentEventMatcher for example)
// and correlates them based on the sequence into a pair.
func Correlate(origin string, in []EventInfo) []EventInfoCombined {
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

func (p *EventProber) SentBy(ctx context.Context, prefix string) []EventInfoCombined {
	name := p.shortNameToName[prefix]
	store := eventshub.StoreFromContext(ctx, name)

	for _, c := range store.Collected() {
		fmt.Printf("%#v\n", c)
	}

	return Correlate(name, store.Collected())
}

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

func (p *EventProber) LoadFullEvents(count int) {
	for i := 0; i < count; i++ {
		id := uuid.New().String()
		event := cetest.FullEvent()
		event.SetID(id)
		p.ids = append(p.ids, id)
		p.senderOptions = append(p.senderOptions, eventshub.InputEvent(event))
	}
}

func (p *EventProber) LoadMinEvents(count int) {
	for i := 0; i < count; i++ {
		id := uuid.New().String()
		event := cetest.MinEvent()
		event.SetID(id)
		p.ids = append(p.ids, id)
		p.senderOptions = append(p.senderOptions, eventshub.InputEvent(event))
	}
}

func (p *EventProber) EventsFromSVC(svcName, path string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		u, err := svc.Address(ctx, svcName)
		if err != nil {
			t.Error(err)
		}
		u.Path = path
		p.senderOptions = append(p.senderOptions, eventshub.InputYAML(u.String()))
	}
}

func (p *EventProber) AsRef(prefix string) *duckv1.KReference {
	name := p.shortNameToName[prefix]
	return svc.AsRef(name)
}

func (p *EventProber) Rx(prefix string) MatchAssertionBuilder {
	name := p.shortNameToName[prefix]
	return OnStore(name)
}

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

func (p *EventProber) AssertReceivedAll(fromPrefix, toPrefix string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {

	}
}

func foo(ctx context.Context, t feature.T) {

}

func (p *EventProber) TriggerSubscriberCfg(prefix string) manifest.CfgFn {
	return trigger.WithSubscriber(p.AsRef(prefix), "")
}

func (p *EventProber) DeadLetterSinkCfg(prefix string) manifest.CfgFn {
	return delivery.WithDeadLetterSink(p.AsRef(prefix), "")
}
