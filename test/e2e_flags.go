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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testflags "knative.dev/eventing/test/flags"
	testlib "knative.dev/eventing/test/lib"
)

const (
	ChannelUsage = "The names of the channel type metas, separated by comma. " +
		`Example: "messaging.knative.dev/v1:InMemoryChannel,` +
		`messaging.knative.dev/v1beta1:KafkaChannel".`
	BrokerClassUsage = "Which brokerclass to test, requires the proper Broker " +
		"implementation to have been installed, and only one value. brokerclass " +
		"must be (for now) 'MTChannelBasedBroker'."
	SourceUsage = "The names of the source type metas, separated by comma. " +
		`Example: "sources.knative.dev/v1:ApiServerSource,` +
		`sources.knative.dev/v1:PingSource".`
	BrokerUsage = "The name of the broker type metas, separated by comma. " +
		`Example: "eventing.knative.dev/v1:MTChannelBasedBroker`
	BrokerNameUsage = "When testing a pre-existing broker, specify the Broker name so the conformance tests " +
		"won't create their own."
	BrokerNamespaceUsage = "When testing a pre-existing broker, this variable specifies the namespace the broker can be found in."
	BrokerClass          = "MTChannelBasedBroker"
)

// EventingFlags holds the command line flags specific to knative/eventing.
var EventingFlags testflags.EventingEnvironmentFlags

// InitializeEventingFlags registers flags used by e2e tests.
func InitializeEventingFlags() {
	// Default value for Channels.
	EventingFlags.Channels = []metav1.TypeMeta{testlib.DefaultChannel}
	flag.Var(&EventingFlags.Channels, "channels", ChannelUsage)

	flag.Var(&EventingFlags.Sources, "sources", SourceUsage)
	flag.StringVar(&EventingFlags.PipeFile, "pipefile", "/tmp/prober-signal", "Temporary file to write the prober signal into.")
	flag.StringVar(&EventingFlags.ReadyFile, "readyfile", "/tmp/prober-ready", "Temporary file to get the prober result.")
	flag.Var(&EventingFlags.Brokers, "brokers", BrokerUsage)
	flag.StringVar(&EventingFlags.BrokerName, "brokername", "", BrokerNameUsage)
	flag.StringVar(&EventingFlags.BrokerNamespace, "brokernamespace", "", BrokerNamespaceUsage)
	// Might be useful in restricted environments where namespaces need to be
	// created by a user with increased privileges (admin).
	flag.BoolVar(&EventingFlags.ReuseNamespace, "reusenamespace", false, "Whether to re-use namespace for a test if it already exists.")
}
