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
	"fmt"
	"log"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/test/lib"
)

// EventingFlags holds the command line flags specific to knative/eventing.
var EventingFlags *EventingEnvironmentFlags

// Channels holds the Channels we want to run test against.
type Channels []metav1.TypeMeta

func (channels *Channels) String() string {
	return fmt.Sprint(*channels)
}

// Set converts the input string to Channels.
// The default Channel we will test against is InMemoryChannel.
func (channels *Channels) Set(value string) error {
	for _, channel := range strings.Split(value, ",") {
		channel := strings.TrimSpace(channel)
		split := strings.Split(channel, ":")
		if len(split) != 2 {
			log.Fatalf("The given Channel name %q is invalid, it needs to be in the form \"apiVersion:Kind\".", channel)
		}
		tm := metav1.TypeMeta{
			APIVersion: split[0],
			Kind:       split[1],
		}
		if !isValid(tm.Kind) {
			log.Fatalf("The given channel name %q is invalid, tests cannot be run.\n", channel)
		}

		*channels = append(*channels, tm)
	}
	return nil
}

// Check if the channel name is valid.
func isValid(channel string) bool {
	return strings.HasSuffix(channel, "Channel")
}

// EventingEnvironmentFlags holds the e2e flags needed only by the eventing repo.
type EventingEnvironmentFlags struct {
	Channels
}

// InitializeEventingFlags registers flags used by e2e tests, calling flag.Parse() here would fail in
// go1.13+, see https://github.com/knative/test-infra/issues/1329 for details
func InitializeEventingFlags() {
	f := EventingEnvironmentFlags{}

	flag.Var(&f.Channels, "channels", "The names of the channel type metas, separated by comma. Example: \"messaging.knative.dev/v1alpha1:InMemoryChannel,messaging.cloud.google.com/v1alpha1:Channel,messaging.knative.dev/v1alpha1:KafkaChannel\".")
	flag.Parse()

	// If no channel is passed through the flag, initialize it as the DefaultChannel.
	if f.Channels == nil || len(f.Channels) == 0 {
		f.Channels = []metav1.TypeMeta{lib.DefaultChannel}
	}

	EventingFlags = &f
}
