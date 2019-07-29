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
	"strings"

	"log"

	"github.com/knative/eventing/test/common"
	pkgTest "knative.dev/pkg/test"
	testLogging "knative.dev/pkg/test/logging"
)

// EventingFlags holds the command line flags specific to knative/eventing.
var EventingFlags = initializeEventingFlags()

// Channels holds the Channels we want to run test against.
type Channels []string

func (channels *Channels) String() string {
	return fmt.Sprint(*channels)
}

// Set converts the input string to Channels.
// The default Channel we will test against is InMemoryChannel.
func (channels *Channels) Set(value string) error {
	// We'll test against all valid channels if we pass "all" through the flag.
	if value == "all" {
		for channel := range common.ValidChannelsMap {
			*channels = append(*channels, channel)
		}
		return nil
	}

	for _, channel := range strings.Split(value, ",") {
		channel := strings.TrimSpace(channel)
		if !isValid(channel) {
			log.Fatalf("The given channel %q is not supported, tests cannot be run.\n", channel)
		}

		*channels = append(*channels, channel)
	}
	return nil
}

// Check if the channel is a valid one.
func isValid(channel string) bool {
	if _, ok := common.ValidChannelsMap[channel]; ok {
		return true
	}
	return false
}

// EventingEnvironmentFlags holds the e2e flags needed only by the eventing repo.
type EventingEnvironmentFlags struct {
	Channels
}

func initializeEventingFlags() *EventingEnvironmentFlags {
	f := EventingEnvironmentFlags{}

	flag.Var(&f.Channels, "channels", "The names of the channels, which are separated by comma.")

	flag.Parse()

	// If no provisioner is passed through the flag, initialize it as the DefaultChannel.
	if f.Channels == nil || len(f.Channels) == 0 {
		f.Channels = []string{common.DefaultChannel}
	}

	testLogging.InitializeLogger(pkgTest.Flags.LogVerbose)
	if pkgTest.Flags.EmitMetrics {
		testLogging.InitializeMetricExporter("eventing")
	}

	return &f
}
