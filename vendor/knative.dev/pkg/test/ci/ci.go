/*
Copyright 2019 The Knative Authors

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

package ci

import (
	"log"
	"os"
	"strings"
)

const (
	// ArtifactsDir is the dir containing artifacts
	ArtifactsDir = "artifacts"
)

// IsCI returns whether the current environment is a CI environment.
func IsCI() bool {
	return strings.EqualFold(os.Getenv("CI"), "true")
}

// GetLocalArtifactsDir gets the artifacts directory where prow looks for artifacts.
// By default, it will look at the env var ARTIFACTS.
func GetLocalArtifactsDir() string {
	dir := os.Getenv("ARTIFACTS")
	if dir == "" {
		log.Printf("Env variable ARTIFACTS not set. Using %s instead.", ArtifactsDir)
		dir = ArtifactsDir
	}
	return dir
}
