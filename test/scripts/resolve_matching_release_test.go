/*
 * Copyright 2020 The Knative Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"testing"

	"github.com/rogpeppe/go-internal/semver"
)

func TestResolveMatchingRelease(t *testing.T) {
	candidate := "v0.8.0"
	resolved := resolveMatchingRelease("knative/serving", candidate)

	if resolved != "v0.8.1" {
		t.Errorf("resolved invalid version: %s", resolved)
	}
}

func TestResolveMatchingReleaseOurAhead(t *testing.T) {
	candidate := "v0.800.0"
	resolved := resolveMatchingRelease("knative/serving", candidate)

	if semver.Compare(resolved, "v1.0.0") >= 0 {
		t.Errorf("resolved invalid version: %s", resolved)
	}
}

func TestResolveIstioRelease(t *testing.T) {
	candidate := "v1.4.0"
	repo := "istio/istio"
	resolved := resolveMatchingRelease(repo, candidate)

	if semver.Compare(resolved, "v1.4.0") < 0 || semver.Compare(resolved, "v1.5.0") >= 0{
		t.Errorf("resolved invalid version: %s", resolved)
	}
}

func TestResolveIstioReleaseWithoutV(t *testing.T) {
	candidate := "1.4.0"
	repo := "istio/istio"
	resolved := resolveMatchingRelease(repo, candidate)

	if semver.Compare(resolved, "v1.4.0") < 0 || semver.Compare(resolved, "v1.5.0") >= 0{
		t.Errorf("resolved invalid version: %s", resolved)
	}
}
