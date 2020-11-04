/*
Copyright 2020 The Knative Authors

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
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"knative.dev/eventing/test/upgrade/prober/wathola/config"
	"knative.dev/eventing/test/upgrade/prober/wathola/fetcher"
	"knative.dev/eventing/test/upgrade/prober/wathola/receiver"
)

func TestFetcherMain(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		st := receiver.Report{
			State:  "active",
			Events: 1456,
			Thrown: []string{},
		}
		bytes, err := json.Marshal(st)
		if err != nil {
			t.Fatal(err)
		}
		_, err = w.Write(bytes)
		if err != nil {
			t.Fatal(err)
		}
	}))
	oldTarget := config.Instance.Forwarder.Target
	config.Instance.Forwarder.Target = ts.URL
	defer ts.Close()
	defer func() {
		config.Instance.Forwarder.Target = oldTarget
	}()

	stdout := func() []byte {
		rescueStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		main()

		_ = w.Close()
		out, _ := ioutil.ReadAll(r)
		os.Stdout = rescueStdout
		return out
	}()

	exec := fetcher.Execution{}
	err := json.Unmarshal(stdout, &exec)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1456, exec.Report.Events)
	assert.Equal(t, "active", exec.Report.State)
}
