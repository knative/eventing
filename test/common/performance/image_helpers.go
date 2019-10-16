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

package performance

import (
	"flag"
	"log"
	"strings"
	"sync"

	"knative.dev/eventing/test/common/performance/aggregator"
	"knative.dev/eventing/test/common/performance/receiver"
	"knative.dev/eventing/test/common/performance/sender"
	"knative.dev/pkg/signals"
)

//go:generate protoc -I ./event_state --go_out=plugins=grpc:./event_state ./event_state/event_state.proto

var (
	roles string

	// role=sender
	aggregAddr    string
	msgSize       uint
	paceFlag      string
	warmupSeconds uint

	// role=aggregator
	expectRecords uint
	listenAddr    string
	makoTags      string
	benchmarkKey  string
	benchmarkName string
)

func DeclareFlags() {
	flag.StringVar(&roles, "roles", "", `Role of this instance. One or multiple (comma-separated) of ("sender", "receiver", "aggregator")`)

	// receiver & sender flags
	flag.StringVar(&paceFlag, "pace", "", "Pace array comma separated. Format rps[:duration=10s]. Example 100,200:4,100:1,500:60")

	// sender flags
	flag.StringVar(&aggregAddr, "aggregator", "", "The aggregator address for sending events records.")
	flag.UintVar(&msgSize, "msg-size", 100, "The size in bytes of each message we want to send. Generate random strings to avoid caching.")
	flag.UintVar(&warmupSeconds, "warmup", 10, "Duration in seconds of warmup phase. During warmup latencies are not recorded. 0 means no warmup")

	// aggregator flags
	flag.StringVar(&listenAddr, "listen-address", ":10000", "Network address the aggregator listens on.")
	flag.UintVar(&expectRecords, "expect-records", 2, "Number of expected events records before aggregating data.")
	flag.StringVar(&makoTags, "mako-tags", "", "Comma separated list of benchmark specific Mako tags.")
	flag.StringVar(&benchmarkKey, "benchmark-key", "TODO", "Benchmark key")
	flag.StringVar(&benchmarkName, "benchmark-name", "TODO", "Benchmark name")
}

func StartPerformanceImage(factory sender.LoadGeneratorFactory, typeExtractor receiver.TypeExtractor, idExtractor receiver.IdExtractor) {
	// We want this for properly handling Kubernetes container lifecycle events.
	ctx := signals.NewContext()

	if roles == "" {
		panic("--roles not set!")
	}

	waitingExecutors := sync.WaitGroup{}

	if strings.Contains(roles, "receiver") {
		if paceFlag == "" {
			panic("--pace not set!")
		}
		if aggregAddr == "" {
			panic("--aggregator not set!")
		}

		log.Println("Creating a receiver")

		receiver, err := receiver.NewReceiver(paceFlag, aggregAddr, typeExtractor, idExtractor)
		if err != nil {
			panic(err)
		}

		waitingExecutors.Add(1)
		go func() {
			receiver.Run(ctx)
			waitingExecutors.Done()
		}()
	}

	if strings.Contains(roles, "sender") {
		if paceFlag == "" {
			panic("--pace not set!")
		}
		if aggregAddr == "" {
			panic("--aggregator not set!")
		}

		log.Println("Creating a sender")

		sender, err := sender.NewSender(factory, aggregAddr, msgSize, warmupSeconds, paceFlag)
		if err != nil {
			panic(err)
		}

		waitingExecutors.Add(1)
		go func() {
			sender.Run(ctx)
			waitingExecutors.Done()
		}()
	}

	if strings.Contains(roles, "aggregator") {
		log.Println("Creating an aggregator")

		aggr, err := aggregator.NewAggregator(benchmarkKey, benchmarkName, listenAddr, expectRecords, strings.Split(makoTags, ","))
		if err != nil {
			panic(err)
		}

		waitingExecutors.Add(1)
		go func() {
			aggr.Run(ctx)
			waitingExecutors.Done()
		}()
	}

	waitingExecutors.Wait()

	log.Println("Performance image completed")
}
