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

package infra

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"knative.dev/pkg/signals"
	pkgtest "knative.dev/pkg/test"

	"knative.dev/eventing/test/performance/infra/aggregator"
	"knative.dev/eventing/test/performance/infra/common"
	"knative.dev/eventing/test/performance/infra/receiver"
	"knative.dev/eventing/test/performance/infra/sender"
)

//go:generate protoc -I ./event_state --go_out=plugins=grpc:./event_state ./event_state/event_state.proto

var (
	roles string

	// role=sender
	aggregAddr    string
	msgSize       uint
	paceFlag      string
	warmupSeconds uint
	fixedBody     bool

	// role=aggregator
	expectRecords uint
	listenAddr    string
	makoTags      string
	publish       bool
)

const (
	defaultTestNamespace = "default"
	podNamespaceEnvVar   = "POD_NAMESPACE"
)

func DeclareFlags() {
	flag.StringVar(&roles, "roles", "", `Role of this instance. One or multiple (comma-separated) of ("sender", "receiver", "aggregator")`)

	// receiver & sender flags
	flag.StringVar(&paceFlag, "pace", "", "Pace array comma separated. Format rps[:duration=10s]. Example 100,200:4,100:1,500:60")

	// sender flags
	flag.StringVar(&aggregAddr, "aggregator", "", "The aggregator address for sending events records.")
	flag.UintVar(&msgSize, "msg-size", 100, "The size in bytes of each message we want to send. Generate random strings to avoid caching.")
	flag.UintVar(&warmupSeconds, "warmup", 10, "Duration in seconds of warmup phase. During warmup latencies are not recorded. 0 means no warmup")
	flag.BoolVar(&fixedBody, "generate-payload-on-each-request", true, "Produce unique body contents for each call")

	// aggregator flags
	flag.StringVar(&listenAddr, "listen-address", ":10000", "Network address the aggregator listens on.")
	flag.UintVar(&expectRecords, "expect-records", 2, "Number of expected events records before aggregating data.")
	flag.StringVar(&makoTags, "mako-tags", "", "Comma separated list of benchmark specific Mako tags.")
	flag.BoolVar(&publish, "publish", true, "Publish the results to mako-stub (default true)")
}

func StartPerformanceImage(factory sender.LoadGeneratorFactory, typeExtractor receiver.TypeExtractor, idExtractor receiver.IdExtractor) {
	// We want this for properly handling Kubernetes container lifecycle events.
	ctx := signals.NewContext()

	if roles == "" {
		panic("--roles not set!")
	}

	var execs []common.Executor

	if strings.Contains(roles, "receiver") {
		if paceFlag == "" {
			panic("--pace not set!")
		}
		if aggregAddr == "" {
			panic("--aggregator not set!")
		}

		log.Println("Creating a receiver")

		receiver, err := receiver.NewReceiver(paceFlag, aggregAddr, warmupSeconds, typeExtractor, idExtractor)
		if err != nil {
			panic(err)
		}

		execs = append(execs, receiver)
	}

	if strings.Contains(roles, "sender") {
		if paceFlag == "" {
			panic("--pace not set!")
		}
		if aggregAddr == "" {
			panic("--aggregator not set!")
		}

		log.Println("Creating a sender")

		sender, err := sender.NewSender(factory, aggregAddr, msgSize, warmupSeconds, paceFlag, fixedBody)
		if err != nil {
			panic(err)
		}

		execs = append(execs, sender)
	}

	if strings.Contains(roles, "aggregator") {
		log.Println("Creating an aggregator")

		aggr, err := aggregator.NewAggregator(listenAddr, expectRecords, strings.Split(makoTags, ","), publish)
		if err != nil {
			panic(err)
		}

		execs = append(execs, aggr)
	}

	// wait until all pods are ready
	ns := testNamespace()
	log.Printf("Waiting for all Pods to be ready in namespace %s", ns)
	if err := waitForPods(ns); err != nil {
		panic(fmt.Errorf("timeout waiting for Pods readiness in namespace %s: %v", ns, err))
	}

	log.Printf("Starting %d executors", len(execs))

	common.Executors(execs).Run(ctx)

	log.Println("Performance image completed")
}

func testNamespace() string {
	if pn := os.Getenv(podNamespaceEnvVar); pn != "" {
		return pn
	}
	return defaultTestNamespace
}

func waitForPods(namespace string) error {
	c, err := pkgtest.NewKubeClient("", "")
	if err != nil {
		return err
	}

	return pkgtest.WaitForAllPodsRunning(context.Background(), c, namespace)
}
