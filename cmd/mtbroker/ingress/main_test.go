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

// An attempt to reproduce knative/eventing#4645

package main

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"knative.dev/pkg/signals"
)

func TestInconsistentPrometheusMetrics(t *testing.T) {
	const concurrentTestSenders = 16

	var pingEvent1Bytes = []byte(`{"data":{"hello":"world"},"datacontenttype":"application/json",` +
		`"id":"837a1976-7a6b-4f7f-ac66-038142900355","source":"/apis/v1/namespaces/default/pingsources/hello",` +
		`"specversion":"1.0","type":"dev.knative.sources.ping"}`)

	var pingEvent2Bytes = []byte(`{"data":{"foo":"bar"},"datacontenttype":"application/json",` +
		`"id":"837a1976-7a6b-4f7f-ac66-038142900355","source":"/apis/v1/namespaces/default/pingsources/cron",` +
		`"specversion":"1.0","type":"dev.knative.sources.ping"}`)

	var heartbeatEvent1Bytes = []byte(`{"data":{"id":545074,"label":""},"datacontenttype":"application/json",` +
		`"id":"837a1976-7a6b-4f7f-ac66-038142900355","source":"https://knative.dev/eventing-contrib/cmd/heartbeats/#default/hb-0123-456",` +
		`"the":"42","heart":"yes","beats":"true",` +
		`"specversion":"1.0","type":"dev.knative.eventing.samples.heartbeat"}`)

	var heartbeatEvent2Bytes = []byte(`{"data":{"id":4688462,"label":""},"datacontenttype":"application/json",` +
		`"id":"837a1976-7a6b-4f7f-ac66-038142900355","source":"https://knative.dev/eventing-contrib/cmd/heartbeats/#default/hb-abcd-efg",` +
		`"the":"42","heart":"yes","beats":"true",` +
		`"specversion":"1.0","type":"dev.knative.eventing.samples.heartbeat"}`)

	var drillEventBytes = []byte(`{"data":"` + strings.Repeat("0", 2048) + `","datacontenttype":"application/json",` +
		`"id":"837a1976-7a6b-4f7f-ac66-038142900355","source":"cegen",` +
		`"specversion":"1.0","type":"io.triggermesh.perf.drill"}`)

	ctx, cancel := context.WithTimeout(signals.NewContext(), 59*time.Minute)
	defer cancel()

	os.Setenv("SYSTEM_NAMESPACE", "knative-eventing")
	os.Setenv("CONTAINER_NAME", "ingress")
	os.Setenv("POD_NAME", "mt-broker-ingress-5f58776864-zsd4p")
	os.Setenv("METRICS_DOMAIN", "issue4645.example.com")

	const scrapeInterval = 3 * time.Second

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		run(ctx)
	}()
	time.Sleep(3 * time.Second)

	// scrape metrics regularly (in prod: every 30s) to invoke the gatherer
	// and trigger a metric consistency check, which we expect to fail
	// eventually
	tickScape := time.Tick(scrapeInterval)
	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			select {
			case <-ctx.Done():
				return

			case <-tickScape:
				resp, err := http.Get("http://127.0.0.1:9092/metrics")
				if err != nil {
					t.Log("Error scraping metrics:", err)
					continue
				}
				resp.Body.Close()
			}
		}
	}()
	time.Sleep(1 * time.Second)

	// simulate ping source (in prod: every 1m)
	tickPing := time.Tick(2 * scrapeInterval)
	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			select {
			case <-ctx.Done():
				return

			case <-tickPing:
				go func() {
					resp, err := http.Post("http://127.0.0.1:8080/default/events",
						cloudevents.ApplicationCloudEventsJSON, bytes.NewReader(pingEvent1Bytes))
					if err != nil {
						t.Log("Error sending fake ping event (hello):", err)
						return
					}
					resp.Body.Close()
				}()

				go func() {
					resp, err := http.Post("http://127.0.0.1:8080/default/default",
						cloudevents.ApplicationCloudEventsJSON, bytes.NewReader(pingEvent2Bytes))
					if err != nil {
						t.Log("Error sending fake ping event (cron):", err)
						return
					}
					resp.Body.Close()
				}()
			}
		}
	}()

	// simulate heartbeat source (in prod: every 1s)
	tickHeartbeat := time.Tick(1 * time.Second)
	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			select {
			case <-ctx.Done():
				return

			case <-tickHeartbeat:
				go func() {
					resp, err := http.Post("http://127.0.0.1:8080/default/events",
						cloudevents.ApplicationCloudEventsJSON, bytes.NewReader(heartbeatEvent1Bytes))
					if err != nil {
						t.Log("Error sending heartbeat event (1):", err)
						return
					}
					resp.Body.Close()
				}()

				go func() {
					resp, err := http.Post("http://127.0.0.1:8080/default/events",
						cloudevents.ApplicationCloudEventsJSON, bytes.NewReader(heartbeatEvent2Bytes))
					if err != nil {
						t.Log("Error sending heartbeat event (2):", err)
						return
					}
					resp.Body.Close()
				}()
			}
		}
	}()

	// simulate load test
	for i := 0; i < concurrentTestSenders; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return

				default:
					resp, err := http.Post("http://127.0.0.1:8080/default/default",
						cloudevents.ApplicationCloudEventsJSON, bytes.NewReader(drillEventBytes))
					if err != nil {
						t.Log("Error sending fake drill event:", err)
						continue
					}
					resp.Body.Close()

					time.Sleep(1 * time.Millisecond)
				}
			}
		}()
	}

	wg.Wait()
}
