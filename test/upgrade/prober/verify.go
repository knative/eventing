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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/eventing/test/upgrade/prober/wathola/receiver"
)

const (
	fetchReportInterval = time.Second
	fetchReportTimeout  = 5 * time.Minute
)

// Verify will verify prober state after finished event has been sent.
func (p *prober) Verify() (eventErrs []error, eventsSent int) {
	var (
		report *receiver.Report
		err error
	)
	start := time.Now()
	if fetchErr := wait.PollImmediate(fetchReportInterval, fetchReportTimeout, func() (bool, error) {
		report, err = p.fetchReport()
		if err != nil {
			return false, err
		}
		if report.State == "active" {
			// Report fetched too early. Trying again.
			return false, nil
		}
		return true, nil
	}); fetchErr != nil {
		p.client.T.Fatalf("Error fetching complete (inactive) report: %v\nReport: %+v", fetchErr, report)
	}
	elapsed := time.Now().Sub(start)
	availRate := 0.0
	if report.TotalRequests != 0 {
		availRate = float64(report.EventsSent*100) / float64(report.TotalRequests)
	}
	p.log.Infof("Fetched receiver report after %s. Events propagated: %v. State: %v.",
		elapsed, report.EventsSent, report.State)
	p.log.Infof("Availability: %.3f%%, Requests sent: %d.",
		availRate, report.TotalRequests)
	for _, t := range report.Thrown.Missing {
		eventErrs = append(eventErrs, errors.New(t))
	}
	for _, t := range report.Thrown.Unexpected {
		eventErrs = append(eventErrs, errors.New(t))
	}
	for _, t := range report.Thrown.Unavailable {
		eventErrs = append(eventErrs, errors.New(t))
	}
	for _, t := range report.Thrown.Duplicated {
		if p.config.OnDuplicate == Warn {
			p.log.Warn("Duplicate events: ", t)
		} else if p.config.OnDuplicate == Error {
			eventErrs = append(eventErrs, errors.New(t))
		}
	}
	return eventErrs, report.EventsSent
}

// Finish terminates sender which sends finished event.
func (p *prober) Finish() {
	p.removeSender()
}

func (p *prober) fetchReport() (*receiver.Report, error) {
	host, err := p.getReceiverServiceHost()
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(fmt.Sprintf("http://%s", host))
	if err != nil {
		return nil, err
	}
	u.Path = "/report"
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("content-type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	report := &receiver.Report{
		Thrown: receiver.Thrown{
			Unexpected:  []string{},
			Duplicated:  []string{},
			Missing:     []string{},
			Unavailable: []string{},
		},
	}
	err = json.Unmarshal(body, report)
	if err != nil {
		return nil, err
	}
	return report, nil
}
