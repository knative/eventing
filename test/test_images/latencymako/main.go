/*
Copyright 2019 The Knative Authors

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

//go:generate protoc -I ./event_state --go_out=plugins=grpc:./event_state ./event_state/event_state.proto

package main

import (
	"context"
	"flag"
	"log"
	"net"

	"google.golang.org/grpc"

	pb "knative.dev/eventing/test/test_images/latencymako/event_state"
	"knative.dev/pkg/signals"
)

// flags for the image
var (
	role    string
	verbose bool

	// role=sender-receiver
	sinkURL       string
	aggregAddr    string
	msgSize       int
	workers       uint64
	paceFlag      string
	warmupSeconds uint

	// role=aggregator
	expectRecords uint
	listenAddr    string
)

func init() {
	flag.StringVar(&role, "role", "", `Role of this instance. One of ("sender-receiver", "aggregator")`)
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")

	// role=sender-receiver
	flag.StringVar(&sinkURL, "sink", "", "The sink URL for the event destination.")
	flag.StringVar(&aggregAddr, "aggregator", "", "The aggregator address for sending events records.")
	flag.IntVar(&msgSize, "msg-size", 100, "The size in bytes of each message we want to send. Generate random strings to avoid caching.")
	flag.UintVar(&warmupSeconds, "warmup", 10, "Duration in seconds of warmup phase. During warmup latencies are not recorded. 0 means no warmup")
	flag.StringVar(&paceFlag, "pace", "", "Pace array comma separated. Format rps[:duration=10s]. Example 100,200:4,100:1,500:60")
	flag.Uint64Var(&workers, "workers", 1, "Number of vegeta workers")

	// role=aggregator
	flag.StringVar(&listenAddr, "listen-address", ":10000", "Network address the aggregator listens on.")
	flag.UintVar(&expectRecords, "expect-records", 1, "Number of expected events records before aggregating data.")
}

type testExecutor interface {
	Run(context.Context)
}

func main() {
	flag.Parse()

	var exec testExecutor

	switch role {

	case "sender-receiver":
		if paceFlag == "" {
			fatalf("pace not set!")
		}
		if sinkURL == "" {
			fatalf("sink not set!")
		}
		if aggregAddr == "" {
			fatalf("aggregator not set!")
		}

		pacerSpecs, err := parsePaceSpec(paceFlag)
		if err != nil {
			fatalf("Failed to parse pace spec: %v", err)
		}

		// wait until all pods are ready (channel, consumers) to ensure events aren't dropped by the broker
		// during the test and the GRPC client can connect to the aggregator
		ns := testNamespace()
		printf("Waiting for all Pods to be ready in namespace %s", ns)
		if err := waitForPods(ns); err != nil {
			fatalf("Timeout waiting for Pods readiness in namespace %s: %v", ns, err)
		}

		// create a connection to the aggregator
		conn, err := grpc.Dial(aggregAddr, grpc.WithInsecure())
		if err != nil {
			fatalf("Failed to connect to the aggregator: %v", err)
		}
		defer conn.Close()

		aggCli := pb.NewEventsRecorderClient(conn)

		exec = newSenderReceiverExecutor(pacerSpecs, aggCli)

	case "aggregator":
		l, err := net.Listen("tcp", listenAddr)
		if err != nil {
			fatalf("Failed to create listener: %v", err)
		}

		exec = newAggregatorExecutor(l)

	default:
		fatalf("Invalid role %q", role)
	}

	// We want this for properly handling Kubernetes container lifecycle events.
	ctx := signals.NewContext()

	exec.Run(ctx)
}

func printf(f string, args ...interface{}) {
	if verbose {
		log.Printf(f, args...)
	}
}

var fatalf = log.Fatalf
