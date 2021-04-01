package eventprober

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/reconciler-test/pkg/environment"

	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/test/rekt/resources/delivery"
	"knative.dev/eventing/test/rekt/resources/svc"
	"knative.dev/eventing/test/rekt/resources/trigger"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/manifest"
	eventshubmain "knative.dev/reconciler-test/pkg/test_images/eventshub"

	. "knative.dev/reconciler-test/pkg/eventshub/assert"
)

// Phases:
// 	Setup
//	Requirement
//	Assert
//	Teardown

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
	Sent     eventshubmain.EventInfo
	Response eventshubmain.EventInfo
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
func Correlate(origin string, in []eventshubmain.EventInfo) []EventInfoCombined {
	var out []EventInfoCombined
	// not too many events, this will suffice...
	for i, e := range in {
		if e.Origin == origin && e.Kind == eventshubmain.EventSent {
			looking := e.Sequence
			for j := i + 1; j <= len(in)-1; j++ {
				if in[j].Kind == eventshubmain.EventResponse && in[j].Sequence == looking {
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

func (p *EventProber) AsRef(prefix string) *duckv1.KReference {
	name := p.shortNameToName[prefix]
	return svc.AsRef(name)
}

func (p *EventProber) Rx(prefix string) MatchAssertionBuilder {
	name := p.shortNameToName[prefix]
	return OnStore(name)
}

func (p *EventProber) AssertSentAll(prefix string) feature.StepFn {
	return func(ctx context.Context, t feature.T) {
		events := p.SentBy(ctx, prefix)
		if len(p.ids) != len(events) {
			t.Errorf("expected %q to have sent %d events, actually sent %d",
				prefix, len(p.ids), len(events))
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

func (p *EventProber) AssertReceivedAll(prefix string) feature.StepFn {
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

// [Source] ---> { Target(Broker)}--> [Sink1] --> {reply}
//                  	|
//   	                +-(DLQ)-> [Sink2]
//
//func SourceToSinkWithDLQ(brokerName string) *feature.Feature {
//	prober := New(broker.GVR(), brokerName, "")
//
//	prober.LoadFullEvents(3)
//
//	via := feature.MakeRandomK8sName("via")
//
//	f := new(feature.Feature)
//
//	// Setup Probes
//	f.Setup("install sink", prober.RxInstall("sink1"))
//	//f.Setup("install sink", prober.FxInstall("sink3"))
//	f.Setup("install dlq", prober.RxInstall("dlq"))
//
//	// Setup data plane
//	f.Setup("update broker with DLQ", broker.Install(brokerName, prober.DeadLetterSinkCfg("dlq")))
//	f.Setup("install trigger", trigger.Install(via, brokerName, trigger.WithSubscriber(prober.AsRef("dlq"), ""), prober.TriggerSubscriberCfg("sink"), trigger.WithFilter()))
//
//	// Resources ready.
//	f.Setup("trigger goes ready", trigger.IsReady(via))
//
//	// Install events after data plane is ready.
//	f.Setup("install source", prober.TxInstall("source"))
//
//	// Assert events ended up +1.
//	f.Stable("broker with DLQ").
//		Must("deliver event to DLQ", prober.Rx("sink").)
//		//OnStore(dlq).MatchEvent(HasId(event.ID())).Exact(1))
//
//		//[]eventsByWho := prober.SentEvents()
//		prober.WantEventsSeenBy("prefix", )
//
//	f.Assert("all events were accepted", func(ctx context.Context, t feature.T) {
//		prober.
//	})
//
//	return f
//}
//
//// trigger.WithSubscriber(loop.AsRef("dlq"), "")
//// loop.TriggerSubscriberCfg("sink")
//
//// TODO: EventProbe
//// [Source] ---> { Target(Broker)}+----> [Sink1] --> {reply}
////                      |         |
////                      |         +--->[Sink3]
////                  	|
////   	                +-(DLQ)-> [Sink2]
////
//
//func SourceToSinkWithDLQ(brokerName string) *feature.Feature {
//	prober := New(broker.GVR(), brokerName, "")
//
//	loop.LoadFullEvents(3)
//
//	via := feature.MakeRandomK8sName("via")
//
//	f := new(feature.Feature)
//
//	// Setup Probes
//	f.Setup("install prober", prober.Install())
//
//	// Setup data plane
//	f.Setup("update broker with DLQ", broker.Install(brokerName, prober.DeadLetterSinkCfg("dlq")))
//	f.Setup("install trigger", trigger.Install(via, brokerName, trigger.WithSubscriber(loop.AsRef("dlq"), ""),loop.TriggerSubscriberCfg("sink"), trigger.WithFilter()))
//
//	// Resources ready.
//	f.Setup("resources goes ready", trigger.AsRef(via), broker.AsRef(brokerName))
//
//	// Install events after data plane is ready.
//	f.Setup("install source", loop.TxInstall("source"))
//
//	// Assert events ended up +1.
//	f.Stable("broker with DLQ").
//		Must("deliver event to DLQ", loop.Rx("sink").)
//	//OnStore(dlq).MatchEvent(HasId(event.ID())).Exact(1))
//
//	//[]eventsByWho := loop.SentEvents()
//	loop.WantEventsSeenBy("prefix", )
//
//	return f
//}
