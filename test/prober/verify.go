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

package prober

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"knative.dev/eventing/test/lib"
	"net/http"
	"net/url"
	"strings"
)

func (p *prober) figureOutClusterHostname() string {
	u, err := url.Parse(p.client.Config.Host)
	lib.NoError(err)
	return u.Hostname()
}

func (p *prober) Verify() ([]error, int) {
	hostname := p.figureOutClusterHostname()
	report := p.fetchReceiverReport(hostname)
	p.log.Infof("Fetched receiver report. Events propagated: %v. "+
		"State: %v", report.Events, report.State)
	if report.State == "active" {
		panic(errors.New("report fetched to early, receiver is in active state"))
	}
	errs := make([]error, 0)
	for _, t := range report.Thrown {
		errs = append(errs, errors.New(t))
	}
	return errs, report.Events
}

func (p *prober) Finish() {
	p.removeSender()
}

func (p *prober) fetchReceiverReport(hostname string) *Report {
	u := fmt.Sprintf("http://%s:%d/report", hostname, receiverNodePort)
	p.log.Infof("Fetching receiver report at: %v", u)
	resp, err := http.Get(u)
	lib.NoError(err)
	if resp.StatusCode != 200 {
		var b strings.Builder
		lib.NoError(resp.Header.Write(&b))
		headers := b.String()
		panic(fmt.Errorf("could not get receiver report at %v, "+
			"status code: %v, headers: %v", u, resp.StatusCode, headers))
	}
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	lib.NoError(err)
	lib.NoError(resp.Body.Close())
	jsonBytes := buf.Bytes()
	var report Report
	lib.NoError(json.Unmarshal(jsonBytes, &report))
	return &report
}

// Report represents a receiver JSON report
type Report struct {
	State  string   `json:"state"`
	Events int      `json:"events"`
	Thrown []string `json:"thrown"`
}
