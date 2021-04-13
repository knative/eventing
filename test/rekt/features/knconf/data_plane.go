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

package knconf

import (
	"context"
	"errors"

	. "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/test/rekt/resources/addressable"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
)

func AcceptsCEVersions(ctx context.Context, t feature.T, gvr schema.GroupVersionResource, name string) {
	u, err := addressable.Address(ctx, gvr, name)
	if err != nil || u == nil {
		t.Error("failed to get the address of the resource", gvr, name, err)
	}

	opts := []eventshub.EventsHubOption{eventshub.StartSenderToResource(gvr, name)}

	uuids := map[string]string{
		uuid.New().String(): "1.0",
		uuid.New().String(): "0.3",
	}
	for id, version := range uuids {
		// We need to use a different source name, otherwise, it will try to update
		// the pod, which is immutable.
		source := feature.MakeRandomK8sName("source")
		event := FullEvent()
		event.SetID(id)
		event.SetSpecVersion(version)
		opts = append(opts, eventshub.InputEvent(event))

		eventshub.Install(source, opts...)(ctx, t)
		store := eventshub.StoreFromContext(ctx, source)
		// We are looking for two events, one of them is the sent event and the other
		// is Response, so correlate them first. We want to make sure the event was sent and that the
		// response was what was expected.
		events := Correlate(store.AssertAtLeast(t, 2, SentEventMatcher(id)))
		for _, e := range events {
			// Make sure HTTP response code is 2XX
			if e.Response.StatusCode < 200 || e.Response.StatusCode > 299 {
				t.Errorf("Expected statuscode 2XX for sequence %d got %d", e.Response.Sequence, e.Response.StatusCode)
			}
		}
	}
}

type EventInfoCombined struct {
	Sent     eventshub.EventInfo
	Response eventshub.EventInfo
}

func SentEventMatcher(uuid string) func(eventshub.EventInfo) error {
	return func(ei eventshub.EventInfo) error {
		if (ei.Kind == eventshub.EventSent || ei.Kind == eventshub.EventResponse) && ei.SentId == uuid {
			return nil
		}
		return errors.New("no match")
	}
}

// Correlate takes in an array of mixed Sent / Response events (matched with sentEventMatcher for example)
// and correlates them based on the sequence into a pair.
func Correlate(in []eventshub.EventInfo) []EventInfoCombined {
	var out []EventInfoCombined
	// not too many events, this will suffice...
	for i, e := range in {
		if e.Kind == eventshub.EventSent {
			looking := e.Sequence
			for j := i + 1; j <= len(in)-1; j++ {
				if in[j].Kind == eventshub.EventResponse && in[j].Sequence == looking {
					out = append(out, EventInfoCombined{Sent: e, Response: in[j]})
				}
			}
		}
	}
	return out
}
