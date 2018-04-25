/*
Copyright 2018 Google, Inc. All rights reserved.

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

// Implements a process that wraps an event-receiving webhook, an event
// queue, and a sender which delivers events to their eventual destination.
package main

import (
	"context"
	"flag"
	"net/http"
	"time"

	"github.com/golang/glog"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	clientset "github.com/elafros/eventing/pkg/client/clientset/versioned"
	informers "github.com/elafros/eventing/pkg/client/informers/externalversions"
	"github.com/elafros/eventing/pkg/delivery"
	"github.com/elafros/eventing/pkg/delivery/action"
	"github.com/elafros/eventing/pkg/delivery/queue"
	"github.com/elafros/eventing/pkg/signals"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	eventBufferSize   = 100
	senderThreads     = 10
	serverAddress     = ":9090"
	metricsScrapePath = "/metrics"
	debugQueuePath    = "/queue/"
	sendEventPrefix   = "/v1alpha1/"
)

var (
	masterURL  string
	kubeconfig string
)

func main() {
	defer glog.Flush()
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	client, err := clientset.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building clientset: %s", err.Error())
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	informerFactory := informers.NewSharedInformerFactory(client, time.Second*30)
	bindLister := informerFactory.Eventing().V1alpha1().Binds().Lister()

	q := queue.NewInMemoryQueue(eventBufferSize)
	receiver := delivery.NewReceiver(bindLister, q)

	processors := map[string]action.Action{
		action.LoggingActionType: action.NewLoggingAction(),
		action.ElafrosActionType: action.NewElafrosAction(kubeClient, http.DefaultClient),
	}
	sender := delivery.NewSender(q, processors)

	go kubeInformerFactory.Start(stopCh)
	go informerFactory.Start(stopCh)

	// Start sender:
	go func() {
		// We don't expect this to return until stop is called,
		// but if it does, propagate it back.
		glog.Info("Staring event sender")
		if err := sender.Run(senderThreads, stopCh); err != nil {
			glog.Fatalf("Error running controller: %s", err.Error())
		}
	}()

	// Set up HTTP endpoints
	mux := http.NewServeMux()
	glog.Infof("Set up metrics scrape path %s", metricsScrapePath)
	mux.Handle(metricsScrapePath, promhttp.Handler())
	glog.Infof("Set up debug queue path %s", debugQueuePath)
	mux.Handle(debugQueuePath, queue.NewDiagnosticHandler(q))
	glog.Infof("Hosting sendEvent API at prefix %s", sendEventPrefix)
	mux.Handle(sendEventPrefix, receiver)
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		glog.Infof("Unhandled %s to route %q", req.Method, req.URL.Path)
		http.NotFound(w, req)
	})

	// Start the API server
	srv := &http.Server{Addr: serverAddress, Handler: mux}
	go func() {
		glog.Infof("Starting API server at %s", serverAddress)
		if err := srv.ListenAndServe(); err != nil {
			glog.Infof("Httpserver: ListenAndServe() finished with error: %s", err)
		}
	}()

	<-stopCh

	// Close the http server gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
