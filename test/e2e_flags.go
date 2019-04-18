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
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/knative/pkg/logging"
	pkgTest "github.com/knative/pkg/test"
	testLogging "github.com/knative/pkg/test/logging"
)

const (
	// E2ETestNamespacePrefix is the namespace prefix used for running all e2e tests.
	E2ETestNamespacePrefix = "e2e-ns"
	// DefaultClusterChannelProvisioner is the default ClusterChannelProvisioner we will run tests against.
	DefaultClusterChannelProvisioner = "in-memory-channel"
	// DefaultBrokerName is the name of the Broker that is automatically created after the current namespace is labeled.
	DefaultBrokerName = "default"
)

var logger = logging.FromContext(context.Background()).Named("eventing-e2e-testing")

// validProvisioners is a list of provisioners that Eventing currently support.
var validProvisioners = []string{DefaultClusterChannelProvisioner}

func isValid(provisioner string) bool {
	for i := range validProvisioners {
		if provisioner == validProvisioners[i] {
			return true
		}
	}
	return false
}

// EventingFlags holds the command line flags specific to knative/eventing.
var EventingFlags = initializeEventingFlags()

// Provisioners holds the ClusterChannelProvisioners we want to run test against.
type Provisioners []string

func (ps *Provisioners) String() string {
	return fmt.Sprint(*ps)
}

// Set converts the input string to Provisioners.
// The default CCP we will test against is in-memory-channel.
func (ps *Provisioners) Set(value string) error {
	for _, provisioner := range strings.Split(value, ",") {
		provisioner := strings.TrimSpace(provisioner)
		if !isValid(provisioner) {
			logger.Fatalf("The given provisioner %q is not supported, tests cannot be run.\n", provisioner)
		}

		*ps = append(*ps, provisioner)
	}
	return nil
}

// EventingEnvironmentFlags holds the e2e flags needed only by the eventing repo.
type EventingEnvironmentFlags struct {
	Provisioners
	RunFromMain bool
}

func initializeEventingFlags() *EventingEnvironmentFlags {
	f := EventingEnvironmentFlags{}

	flag.Var(&f.Provisioners, "clusterChannelProvisioners", "The names of the Channel's clusterChannelProvisioners, which are separated by comma.")
	flag.BoolVar(&f.RunFromMain, "runFromMain", false, "If runFromMain is set to false, the TestMain will be skipped when we run tests.")

	flag.Parse()

	// If no provisioner is passed through the flag, initialize it as the DefaultClusterChannelProvisioner.
	if f.Provisioners == nil || len(f.Provisioners) == 0 {
		f.Provisioners = []string{DefaultClusterChannelProvisioner}
	}

	// If we are not running from TestMain, only one single provisioner can be specified.
	if !f.RunFromMain && len(f.Provisioners) != 1 {
		logger.Fatal("Only one single provisioner can be specified if you are not running from TestMain.")
	}

	testLogging.InitializeLogger(pkgTest.Flags.LogVerbose)
	if pkgTest.Flags.EmitMetrics {
		testLogging.InitializeMetricExporter("eventing")
	}

	return &f
}
