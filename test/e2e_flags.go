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
	"os"

	pkgTest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
)

// EventingFlags holds the command line flags specific to knative/eventing
var EventingFlags = initializeEventingFlags()

// EventingEnvironmentFlags holds the e2e flags needed only by the eventing repo
type EventingEnvironmentFlags struct {
	DockerRepo  string // Docker repo (defaults to $DOCKER_REPO_OVERRIDE)
	Tag         string // Tag for test images
	Provisioner string // The name of the Channel's ClusterChannelProvisioner
}

func initializeEventingFlags() *EventingEnvironmentFlags {
	var f EventingEnvironmentFlags

	defaultRepo := os.Getenv("DOCKER_REPO_OVERRIDE")

	if defaultRepo == "" {
		defaultRepo = os.Getenv("KO_DOCKER_REPO")
	}

	flag.StringVar(&f.DockerRepo, "dockerrepo", defaultRepo,
		"Provide the uri of the docker repo you have uploaded the test image to using `uploadtestimage.sh`. Defaults to $DOCKER_REPO_OVERRIDE")

	flag.StringVar(&f.Tag, "tag", "e2e", "Provide the version tag for the test images.")

	flag.StringVar(&f.Provisioner, "clusterChannelProvisioner", "in-memory-channel", "The name of the Channel's clusterChannelProvisioner. Only the in-memory-channel is installed by the tests, anything else must be installed before the tests are run.")

	flag.Parse()

	logging.InitializeLogger(pkgTest.Flags.LogVerbose)

	if pkgTest.Flags.EmitMetrics {
		logging.InitializeMetricExporter()
	}

	return &f
}

// ImagePath returns an image path using the configured image repo and tag.
func ImagePath(name string) string {
	return fmt.Sprintf("%s/%s:%s", EventingFlags.DockerRepo, name, EventingFlags.Tag)
}
