/*
Copyright 2018 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/elafros/eventing/pkg/event"
	"github.com/golang/glog"
)

const (
	databaseURL = "https://inlined-junkdrawer.firebaseio.com/"
	address     = ":8080"
)

// SendEventToDB...
func SendEventToDB(w http.ResponseWriter, r *http.Request) {
	glog.Info("Sending event to RTDB")
	var data map[string]interface{}
	context, err := event.FromRequest(&data, r)
	if err != nil {
		glog.Errorf("Failed to parse event: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	url, err := url.Parse(databaseURL + "seenEvents/" + context.EventID + "/.json")
	if err != nil {
		glog.Errorf("Failed to parse url %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// Can not fail
	body, _ := json.Marshal(data)

	write := &http.Request{
		Method: http.MethodPut,
		URL:    url,
		Body:   ioutil.NopCloser(bytes.NewReader(body)),
	}
	res, err := http.DefaultClient.Do(write)
	if err != nil {
		glog.Errorf("Failed to write to RTDB: %s", err)
	}
	if res.StatusCode/100 != 2 {
		glog.Errorf("Got non-success response from RTDB: %d", res.StatusCode)
	}
	glog.Info("Success!")

	w.WriteHeader(http.StatusOK)
}

func main() {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.Write([]byte(`{"status": "ok"}`))
	})
	http.HandleFunc("/", SendEventToDB)
	http.ListenAndServe(address, nil)
}
