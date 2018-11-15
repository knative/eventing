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
	"os"
	"path"

	pkgTest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
)

// EventingFlags holds the command line flags specific to knative/eventing
var EventingFlags = initializeEventingFlags()

// EventingEnvironmentFlags holds the e2e flags needed only by the eventing repo
type EventingEnvironmentFlags struct {
	DockerRepo string // Docker repo (defaults to $DOCKER_REPO_OVERRIDE)
	Tag        string // Tag for test images
}

func initializeEventingFlags() *EventingEnvironmentFlags {
	var f EventingEnvironmentFlags

	defaultRepo := path.Join(os.Getenv("DOCKER_REPO_OVERRIDE"), "github.com/knative/eventing/test/test_images")
	flag.StringVar(&f.DockerRepo, "dockerrepo", defaultRepo,
		"Provide the uri of the docker repo you have uploaded the test image to using `uploadtestimage.sh`. Defaults to $DOCKER_REPO_OVERRIDE")

	flag.StringVar(&f.Tag, "tag", "e2e", "Provide the version tag for the test images.")

	flag.Parse()

	logging.InitializeLogger(pkgTest.Flags.LogVerbose)

	if pkgTest.Flags.EmitMetrics {
		logging.InitializeMetricExporter()
	}

	return &f
}
