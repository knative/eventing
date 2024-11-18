package main

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/validation"

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
