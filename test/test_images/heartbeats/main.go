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
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"knative.dev/eventing/pkg/kncloudevents"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"github.com/kelseyhightower/envconfig"
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

	c, err := kncloudevents.NewDefaultClient(sink)
	if err != nil {
		log.Fatalf("failed to create client: %s", err.Error())
	}

	var period time.Duration
	if p, err := strconv.Atoi(periodStr); err != nil {
		period = time.Duration(5) * time.Second
	} else {
		period = time.Duration(p) * time.Second
	}

	source := types.ParseURLRef(
		fmt.Sprintf("https://knative.dev/eventing/test/heartbeats/#%s/%s", env.Namespace, env.Name))
	log.Printf("Heartbeats Source: %s", source)

	hb := &Heartbeat{
		Sequence: 0,
		Msg:      msg,
	}
	ticker := time.NewTicker(period)
	for {
		hb.Sequence++

		event := cloudevents.Event{
			Context: cloudevents.EventContextV02{
				Type:   "dev.knative.eventing.samples.heartbeat",
				Source: *source,
				Extensions: map[string]interface{}{
					"the":   42,
					"heart": "yes",
					"beats": true,
				},
			}.AsV1(),
			Data: hb,
		}
		event.SetDataContentType(cloudevents.ApplicationJSON)

		log.Printf("sending cloudevent to %s", sink)
		if _, _, err := c.Send(context.Background(), event); err != nil {
			log.Printf("failed to send cloudevent: %s", err.Error())
		}

		if env.OneShot {
			return
		}

		// Wait for next tick
		<-ticker.C
	}
}
