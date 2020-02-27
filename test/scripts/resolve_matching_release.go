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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"

	"github.com/rogpeppe/go-internal/semver"
	"github.com/wavesoftware/go-ensure"
)

var (
	ApiGateway = "https://api.github.com"
)

func main() {
	args := os.Args[1:]
	if len(args) != 2 {
		panic(fmt.Errorf("usage: <repo> <version>"))
	}

	resolved := resolveMatchingRelease(args[0], args[1])
	fmt.Print(resolved)
}

type bySemver []string

func (s bySemver) Len() int {
	return len(s)
}

func (s bySemver) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s bySemver) Less(i, j int) bool {
	return semver.Compare(s[i], s[j]) > 0
}

type Tag struct {
	Name string
}

func resolveMatchingRelease(repo, version string) string {
	version = prependV(version)
	endpoint := fmt.Sprintf("%s/repos/%s/tags", ApiGateway, repo)
	resp, err := http.Get(endpoint)
	ensure.NoError(err)
	defer func() {
		err := resp.Body.Close()
		ensure.NoError(err)
	}()
	body, err := ioutil.ReadAll(resp.Body)
	var tags []Tag
	ensure.NoError(json.Unmarshal(body, &tags))

	versions := slurpVersions(tags)
	matching := filterMatchingVersions(version, versions, true)
	if len(matching) == 0 {
		matching = filterMatchingVersions(version, versions, false)
		if len(matching) == 0 {
			matching = versions
		}
	}
	sort.Sort(bySemver(matching))
	first := matching[0]

	return first
}

func slurpVersions(tags []Tag) []string {
	var res []string
	for _, tag := range tags {
		res = append(res, prependV(tag.Name))
	}
	return res
}

func prependV(version string) string {
	if version[0] != 'v' {
		return "v" + version
	}
	return version
}

func filterMatchingVersions(version string, versions []string, strict bool) []string {
	matcher := semverMatcher(version, strict)
	var matching []string
	for _, candidate := range versions {
		if matcher == semverMatcher(candidate, strict) {
			matching = append(matching, candidate)
		}
	}
	return matching
}

func semverMatcher(version string, strict bool) string {
	if strict {
		return semver.MajorMinor(version)
	}
	return semver.Major(version)
}
