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

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/knative/eventing/pkg/signals"
)

const (
	// Target for messages
	envTarget = "TARGET"
	// interval
	envInterval = "INTERVAL"
	// until
	envUntil = "UNTIL"
)

type message struct {
	Message string
}

func main() {
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	target := os.Getenv(envTarget)
	interval := os.Getenv(envInterval)
	until := os.Getenv(envUntil)

	glog.Infof("Target is: %q, Interval: %q, Until: %q", target, interval, until)

	glog.Infof("Creating a new  Timer...")
	duration, err := time.ParseDuration(interval)
	if err != nil {
		glog.Errorf("failed to parse %q", interval)
	}
	end, err := time.Parse(time.RFC3339, until)
	if err != nil {
		glog.Errorf("failed to parse %q", until)
	}

	tick := time.NewTicker(duration).C

	for {
		select {
		case <-tick:
			postMessage(target)
			if time.Now().After(end) {
				glog.Infof("Alarm ended, exiting...")
				return
			}
		case <-stopCh:
			glog.Infof("Exiting...")
			return
		}
	}
}

func postMessage(target string) error {
	msg := message{
		Message: time.Now().String(),
	}

	jsonStr, err := json.Marshal(msg)
	if err != nil {
		glog.Infof("Failed to marshal the message: %+v : %s", msg, err)
		return err
	}

	URL := fmt.Sprintf("http://%s/", target)
	glog.Infof("Posting to %q", URL)
	req, err := http.NewRequest("POST", URL, bytes.NewBuffer(jsonStr))
	if err != nil {
		glog.Infof("Failed to create http request: %s", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		glog.Infof("Failed to do POST: %v", err)
		return err
	}
	defer resp.Body.Close()
	glog.Infof("response Status: %s", resp.Status)
	body, _ := ioutil.ReadAll(resp.Body)
	glog.Infof("response Body: %s", string(body))
	return nil
}
