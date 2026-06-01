/*
Copyright 2024 The Knative Authors

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

package main

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/types"

	"knative.dev/eventing/pkg/utils"
)

type testCase struct {
	JobSinkName string
	Source      string
	Id          string
}

func TestToJobName(t *testing.T) {
	testcases := []testCase{
		{
			JobSinkName: "job-sink-success",
			Source:      "mysource3/myservice",
			Id:          "2234-5678",
		},
		{
			JobSinkName: "a",
			Source:      "0",
			Id:          "0",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.JobSinkName+"_"+tc.Source+"_"+tc.Id, func(t *testing.T) {
			if errs := validation.NameIsDNS1035Label(tc.JobSinkName, false); len(errs) != 0 {
				t.Errorf("Invalid JobSinkName: %v", errs)
			}

			name := toJobName(tc.JobSinkName, tc.Source, tc.Id)
			doubleName := toJobName(tc.JobSinkName, tc.Source, tc.Id)
			if name != doubleName {
				t.Errorf("Before: %q, after: %q", name, doubleName)
			}

			if got := utils.ToDNS1123Subdomain(name); got != name {
				t.Errorf("ToDNS1123Subdomain(Want) returns a different result, Want: %q, Got: %q", name, got)
			}

			if errs := validation.NameIsDNS1035Label(name, false); len(errs) != 0 {
				t.Errorf("toJobName produced invalid name %q given %q, %q, %q: errors: %#v", name, tc.JobSinkName, tc.Source, tc.Id, errs)
			}
		})
	}
}

func FuzzToJobName(f *testing.F) {
	testcases := []testCase{
		{
			JobSinkName: "job-sink-success",
			Source:      "mysource3/myservice",
			Id:          "2234-5678",
		},
		{
			JobSinkName: "a",
			Source:      "0",
			Id:          "0",
		},
	}

	for _, tc := range testcases {
		f.Add(tc.JobSinkName, tc.Source, tc.Id) // Use f.Add to provide a seed corpus
	}
	f.Fuzz(func(t *testing.T, js, source, id string) {
		if errs := validation.NameIsDNSLabel(js, false); len(errs) != 0 {
			t.Skip("Prerequisite: invalid jobsink name")
		}

		name := toJobName(js, source, id)
		doubleName := toJobName(js, source, id)
		if name != doubleName {
			t.Errorf("Before: %q, after: %q", name, doubleName)
		}

		if got := utils.ToDNS1123Subdomain(name); got != name {
			t.Errorf("ToDNS1123Subdomain(Want) returns a different result, Want: %q, Got: %q", name, got)
		}

		if errs := validation.NameIsDNSLabel(name, false); len(errs) != 0 {
			t.Errorf("toJobName produced invalid name %q given %q, %q, %q: errors: %#v", name, js, source, id, errs)
		}
	})
}

func TestLocationHeaderRoundTrip(t *testing.T) {
	testCases := map[string]struct {
		ref    types.NamespacedName
		source string
		id     string
	}{
		"simple": {
			ref: types.NamespacedName{
				Namespace: "test-namespace",
				Name:      "job-sink",
			},
			source: "mysource3",
			id:     "2234-5678",
		},
		"slashes in source and id": {
			ref: types.NamespacedName{
				Namespace: "test-namespace",
				Name:      "job-sink",
			},
			source: "https://example.com/sources/my/source",
			id:     "event/id/with/slashes",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			location := locationHeader(tc.ref, tc.source, tc.id)

			gotRef, gotSource, gotID, err := parseLocation(location)
			if err != nil {
				t.Fatal("unexpected error:", err)
			}

			if gotRef != tc.ref {
				t.Fatalf("unexpected ref: got %#v want %#v", gotRef, tc.ref)
			}
			if gotSource != tc.source {
				t.Fatalf("unexpected source: got %q want %q", gotSource, tc.source)
			}
			if gotID != tc.id {
				t.Fatalf("unexpected id: got %q want %q", gotID, tc.id)
			}
		})
	}
}

func TestParseLocationRejectsUnescapedSourceSlashes(t *testing.T) {
	_, _, _, err := parseLocation("/namespaces/test-namespace/name/job-sink/sources/https://example.com/source/ids/event-id")
	if err == nil {
		t.Fatal("expected error for malformed location")
	}
}
