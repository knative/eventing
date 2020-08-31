/*
Copyright 2018 The Knative Authors

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

// This file contains logic to encapsulate flags which are needed to specify
// what cluster, etc. to use for e2e tests.

package test

import (
	"flag"
	"log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testflags "knative.dev/eventing/test/flags"
	testlib "knative.dev/eventing/test/lib"
)

const (
	ChannelUsage = "The names of the channel type metas, separated by comma. " +
		"Example: \"messaging.knative.dev/v1alpha1:InMemoryChannel," +
		"messaging.cloud.google.com/v1alpha1:Channel,messaging.knative.dev/v1alpha1:KafkaChannel\"."
	BrokerClassUsage = "Which brokerclass to test, requires the proper Broker " +
		"implementation to have been installed, and only one value. brokerclass " +
		"must be (for now) 'MTChannelBasedBroker'."
	SourceUsage = "The names of the source type metas, separated by comma. " +
		"Example: \"sources.knative.dev/v1alpha1:ApiServerSource," +
		"sources.knative.dev/v1alpha1:PingSource\"."
	BrokerUsage = "The name of the broker type metas, separated by comma. " +
		"Example: \"eventing.knative.dev/v1beta1:MTChannelBasedBroker"
	BrokerNameUsage = "When testing a pre-existing broker, specify the Broker name so the conformance tests " +
		"won't create their own."
	BrokerNamespaceUsage = "When testing a pre-existing broker, this variable specifies the namespace the broker can be found in."
)

// EventingFlags holds the command line flags specific to knative/eventing.
var EventingFlags testflags.EventingEnvironmentFlags

// InitializeEventingFlags registers flags used by e2e tests, calling flag.Parse() here would fail in
// go1.13+, see https://github.com/knative/test-infra/issues/1329 for details
func InitializeEventingFlags() {

	flag.Var(&EventingFlags.Channels, "channels", ChannelUsage)
	flag.StringVar(&EventingFlags.BrokerClass, "brokerclass", "MTChannelBasedBroker", BrokerClassUsage)
	flag.Var(&EventingFlags.Sources, "sources", SourceUsage)
	flag.StringVar(&EventingFlags.PipeFile, "pipefile", "/tmp/prober-signal", "Temporary file to write the prober signal into.")
	flag.StringVar(&EventingFlags.ReadyFile, "readyfile", "/tmp/prober-ready", "Temporary file to get the prober result.")
	flag.Var(&EventingFlags.Brokers, "brokers", BrokerUsage)
	flag.StringVar(&EventingFlags.BrokerName, "brokername", "", BrokerNameUsage)
	flag.StringVar(&EventingFlags.BrokerNamespace, "brokernamespace", "", BrokerNamespaceUsage)
	flag.Parse()

	// If no channel is passed through the flag, initialize it as the DefaultChannel.
	if EventingFlags.Channels == nil || len(EventingFlags.Channels) == 0 {
		EventingFlags.Channels = []metav1.TypeMeta{testlib.DefaultChannel}
	}

	if EventingFlags.BrokerClass == "" {
		log.Fatalf("Brokerclass not specified")
	}

	if EventingFlags.BrokerClass != "MTChannelBasedBroker" {
		log.Fatalf("Invalid Brokerclass specified, got %q must be %q", EventingFlags.BrokerClass, "MTChannelBasedBroker")
	}
}
