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

// Provisioners holds the ClusterChannelProvisioners we want to run test against.
type Provisioners []string

func (ps *Provisioners) String() string {
	return fmt.Sprint(*ps)
}

// Set converts the input string to Provisioners.
// The default CCP we will test against is in-memory.
func (ps *Provisioners) Set(value string) error {
	// We'll test against all valid provisioners if we pass "all" through the flag.
	if value == "all" {
		for provisioner := range common.ValidProvisionersMap {
			*ps = append(*ps, provisioner)
		}
		return nil
	}

	for _, provisioner := range strings.Split(value, ",") {
		provisioner := strings.TrimSpace(provisioner)
		if !isValid(provisioner) {
			log.Fatalf("The given provisioner %q is not supported, tests cannot be run.\n", provisioner)
		}

		*ps = append(*ps, provisioner)
	}
	return nil
}

// Check if the provisioner is a valid one.
func isValid(provisioner string) bool {
	if _, ok := common.ValidProvisionersMap[provisioner]; ok {
		return true
	}
	return false
}

// EventingEnvironmentFlags holds the e2e flags needed only by the eventing repo.
type EventingEnvironmentFlags struct {
	Provisioners
}

func initializeEventingFlags() *EventingEnvironmentFlags {
	f := EventingEnvironmentFlags{}

	flag.Var(&f.Provisioners, "clusterChannelProvisioners", "The names of the Channel's clusterChannelProvisioners, which are separated by comma.")

	flag.Parse()

	// If no provisioner is passed through the flag, initialize it as the DefaultClusterChannelProvisioner.
	if f.Provisioners == nil || len(f.Provisioners) == 0 {
		f.Provisioners = []string{common.DefaultClusterChannelProvisioner}
	}

	testLogging.InitializeLogger(pkgTest.Flags.LogVerbose)
	if pkgTest.Flags.EmitMetrics {
		testLogging.InitializeMetricExporter("eventing")
	}

	return &f
}
