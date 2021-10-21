/*
Copyright 2020 The Knative Authors

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

package trigger

import (
	"fmt"

	"knative.dev/eventing/test/rekt/resources/broker"
	"knative.dev/eventing/test/rekt/resources/delivery"
	"knative.dev/eventing/test/rekt/resources/eventlibrary"
	"knative.dev/eventing/test/rekt/resources/trigger"
	"knative.dev/reconciler-test/pkg/eventshub"
	"knative.dev/reconciler-test/pkg/feature"
)

// SourceToSinkWithDLQ tests to see if a Ready Broker acts as middleware.
//
// source ---> broker<Via> --[trigger]--> bad uri
//                |
//                +--[DLQ]--> sink
//
func SourceToSinkWithDLQ(brokerName string) *feature.Feature {
	prober := eventshub.NewProber()
	prober.SetTargetResource(broker.GVR(), brokerName)

	via := feature.MakeRandomK8sName("via")

	f := feature.NewFeature()

	lib := feature.MakeRandomK8sName("lib")
	f.Setup("install events", eventlibrary.Install(lib))
	f.Setup("event cache is ready", eventlibrary.IsReady(lib))
	f.Setup("use events cache", prober.SenderEventsFromSVC(lib, "events/three.ce"))
	if err := prober.ExpectYAMLEvents(eventlibrary.PathFor("events/three.ce")); err != nil {
		panic(fmt.Errorf("can not find event files: %s", err))
	}

	// Setup Probes
	f.Setup("install recorder", prober.ReceiverInstall("sink"))

	// Setup data plane
	brokerConfig := append(broker.WithEnvConfig(), delivery.WithDeadLetterSink(prober.AsKReference("sink"), ""))
	f.Setup("update broker with DLQ", broker.Install(
		brokerName,
		brokerConfig...,
	))
	f.Setup("install trigger", trigger.Install(via, brokerName, trigger.WithSubscriber(nil, "bad://uri")))

	// Resources ready.
	f.Setup("trigger goes ready", trigger.IsReady(via))

	// Install events after data plane is ready.
	f.Setup("install source", prober.SenderInstall("source"))

	// After we have finished sending.
	f.Requirement("sender is finished", prober.SenderDone("source"))

	// Assert events ended up where we expected.
	f.Stable("broker with DLQ").
		Must("accepted all events", prober.AssertSentAll("source")).
		Must("deliver event to DLQ", prober.AssertReceivedAll("source", "sink"))

	return f
}
