/*
Copyright 2019 The Knative Authors

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
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/kelseyhightower/envconfig"
	"go.opencensus.io/plugin/ochttp"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/tracing/propagation/tracecontextb3"
)

type Heartbeat struct {
	Sequence int    `json:"id"`
	Msg      string `json:"msg"`
}

var (
	sink      string
	msg       string
	periodStr string
)

func init() {
	flag.StringVar(&sink, "sink", "", "the host url to heartbeat to")
	flag.StringVar(&msg, "msg", "", "the data message")
	flag.StringVar(&periodStr, "period", "5", "the number of seconds between heartbeats")
}

type envConfig struct {
	// Sink URL where to send heartbeat cloudevents
	Sink string `envconfig:"K_SINK"`

	// CEOverrides are the CloudEvents overrides to be applied to the outbound event.
	CEOverrides string `envconfig:"K_CE_OVERRIDES"`

	// Name of this pod.
	Name string `envconfig:"POD_NAME" required:"true"`

	// Namespace this pod exists in.
	Namespace string `envconfig:"POD_NAMESPACE" required:"true"`

	OneShot bool `envconfig:"ONE_SHOT" default:"false"`
}

func main() {
	flag.Parse()

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Printf("[ERROR] Failed to process env var: %s", err)
		os.Exit(1)
	}

	if env.Sink != "" {
		sink = env.Sink
	}

	var ceOverrides *duckv1.CloudEventOverrides
	if len(env.CEOverrides) > 0 {
		overrides := duckv1.CloudEventOverrides{}
		err := json.Unmarshal([]byte(env.CEOverrides), &overrides)
		if err != nil {
			log.Printf("[ERROR] Unparseable CloudEvents overrides %s: %v", env.CEOverrides, err)
			os.Exit(1)
		}
		ceOverrides = &overrides
	}

	p, err := cloudevents.NewHTTP(
		cloudevents.WithTarget(sink),
		cloudevents.WithRoundTripper(&ochttp.Transport{
			Propagation: tracecontextb3.TraceContextEgress,
		}))
	if err != nil {
		log.Fatalf("failed to create protocol: %s", err.Error())
	}

	c, err := cloudevents.NewClient(p, cloudevents.WithUUIDs(), cloudevents.WithTimeNow())
	if err != nil {
		log.Fatalf("failed to create client: %s", err.Error())
	}

	var period time.Duration
	if p, err := strconv.Atoi(periodStr); err != nil {
		period = time.Duration(5) * time.Second
	} else {
		period = time.Duration(p) * time.Second
	}

	source := fmt.Sprintf("https://knative.dev/eventing/test/heartbeats/#%s/%s", env.Namespace, env.Name)
	log.Printf("Heartbeats Source: %s", source)

	hb := &Heartbeat{
		Sequence: 0,
		Msg:      msg,
	}
	ticker := time.NewTicker(period)
	for {
		hb.Sequence++

		event := cloudevents.NewEvent()
		event.SetType("dev.knative.eventing.samples.heartbeat")
		event.SetSource(source)
		event.SetExtension("the", 42)
		event.SetExtension("heart", "yes")
		event.SetExtension("beats", true)

		if ceOverrides != nil && ceOverrides.Extensions != nil {
			for n, v := range ceOverrides.Extensions {
				event.SetExtension(n, v)
			}
		}

		if err := event.SetData(cloudevents.ApplicationJSON, hb); err != nil {
			log.Fatalf("failed to set cloudevents data: %s", err.Error())
		}

		log.Printf("sending cloudevent to %s", sink)
		if result := c.Send(context.Background(), event); !cloudevents.IsACK(result) {
			log.Printf("failed to send cloudevent: %s", result.Error())
		}

		if env.OneShot {
			return
		}

		// Wait for next tick
		<-ticker.C
	}
}
