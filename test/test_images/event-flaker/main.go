/*
Copyright 2021 The Knative Authors

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
	"fmt"
	"log"
	"os"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/kelseyhightower/envconfig"
	_ "knative.dev/pkg/system/testing"
)

type envConfig struct {
	Reject int    `envconfig:"REJECT_MOD" default:"3"`
	Sink   string `envconfig:"K_SINK" required:"true"`
}

func main() {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Printf("[ERROR] Failed to process env var: %s", err)
		os.Exit(1)
	}

	p, err := cloudevents.NewHTTP(cloudevents.WithTarget(env.Sink))
	if err != nil {
		log.Fatalf("failed to create protocol: %s", err.Error())
	}

	c, err := cloudevents.NewClient(p, cloudevents.WithUUIDs(), cloudevents.WithTimeNow())
	if err != nil {
		log.Fatalf("failed to create client: %s", err.Error())
	}

	count := 0

	if err := c.StartReceiver(context.Background(), func(ctx context.Context, event cloudevents.Event) error {
		count++
		if count%env.Reject == 0 {
			if result := c.Send(context.Background(), event); !cloudevents.IsACK(result) {
				log.Printf("failed to send cloudevent: %s", result.Error())
			}
			return nil
		}
		return fmt.Errorf("rejected %d", count)
	}); err != nil {
		log.Fatalf("failed to start receiver: %s", err.Error())
	}
}
