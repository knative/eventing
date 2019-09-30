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
	"knative.dev/eventing/test/common/performance/aggregator"
	"knative.dev/eventing/test/common/performance/receiver"
	"knative.dev/eventing/test/common/performance/sender"
	"knative.dev/pkg/signals"
	"log"
	"strings"
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
)

func DeclareFlags() {
	flag.StringVar(&roles, "roles", "", `Role of this instance. One or multiple (comma-separated) of ("sender", "receiver", "aggregator")`)

	// role=receiver & role=receiver
	flag.StringVar(&paceFlag, "pace", "", "Pace array comma separated. Format rps[:duration=10s]. Example 100,200:4,100:1,500:60")

	// role=sender
	flag.StringVar(&aggregAddr, "aggregator", "", "The aggregator address for sending events records.")
	flag.UintVar(&msgSize, "msg-size", 100, "The size in bytes of each message we want to send. Generate random strings to avoid caching.")
	flag.UintVar(&warmupSeconds, "warmup", 10, "Duration in seconds of warmup phase. During warmup latencies are not recorded. 0 means no warmup")

	// role=aggregator
	flag.StringVar(&listenAddr, "listen-address", ":10000", "Network address the aggregator listens on.")
	flag.UintVar(&expectRecords, "expect-records", 2, "Number of expected events records before aggregating data.")
	flag.StringVar(&makoTags, "mako-tags", "", "Comma separated list of benchmark"+
		" specific Mako tags.")
}

func StartPerformanceImage(factory sender.LoadGeneratorFactory) {
	// We want this for properly handling Kubernetes container lifecycle events.
	ctx := signals.NewContext()

	if roles == "" {
		panic("--roles not set!")
	}

	waitingExecutors := 0
	endExecutorCh := make(chan bool)

	if strings.Contains(roles, "receiver") {
		if paceFlag == "" {
			panic("--pace not set!")
		}
		if aggregAddr == "" {
			panic("--aggregator not set!")
		}

		log.Println("Creating a receiver")

		receiver, err := receiver.NewReceiver(paceFlag, aggregAddr)
		if err != nil {
			panic(err)
		}

		waitingExecutors++
		go func() {
			receiver.Run(ctx)
			endExecutorCh <- true
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

		waitingExecutors++
		go func() {
			sender.Run(ctx)
			endExecutorCh <- true
		}()
	}

	if strings.Contains(roles, "aggregator") {
		log.Println("Creating an aggregator")

		aggr, err := aggregator.NewAggregator(listenAddr, expectRecords, strings.Split(makoTags, ","))
		if err != nil {
			panic(err)
		}

		waitingExecutors++
		go func() {
			aggr.Run(ctx)
			endExecutorCh <- true
		}()
	}

	for {
		if waitingExecutors == 0 {
			break
		} else {
			<-endExecutorCh
			waitingExecutors--
			log.Printf("Waiting %d executors\n", waitingExecutors)
		}
	}

	log.Println("Performance image completed")

}
