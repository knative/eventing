/*
 * Copyright 2018 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-martini/martini"
	"github.com/golang/glog"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	clientset "github.com/knative/eventing/pkg/client/clientset/versioned"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"github.com/knative/eventing/pkg/subscription"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	masterURL  string
	kubeconfig string

	broker         = os.Getenv("BROKER_NAME")
	forwardHeaders = []string{
		"content-type",
		"x-request-id",
		"x-b3-traceid",
		"x-b3-spanid",
		"x-b3-parentspanid",
		"x-b3-sampled",
		"x-b3-flags",
		"x-ot-span-context",
	}
)

func splitStreamName(host string) (string, string) {
	chunks := strings.Split(host, ".")
	stream := chunks[0]
	namespace := chunks[1]
	return stream, namespace
}

func main() {
	flag.Parse()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	client, err := clientset.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building clientset: %s", err.Error())
	}

	informerFactory := informers.NewSharedInformerFactory(client, time.Second*30)
	monitor := subscription.NewMonitor(broker, informerFactory, subscription.MonitorEventHandlerFuncs{
		ProvisionFunc: func(stream eventingv1alpha1.Stream) {
			fmt.Printf("Provision stream %q\n", stream.Name)
		},
		UnprovisionFunc: func(stream eventingv1alpha1.Stream) {
			fmt.Printf("Unprovision stream %q\n", stream.Name)
		},
		SubscribeFunc: func(subscription eventingv1alpha1.Subscription) {
			fmt.Printf("Subscribe %q to %q stream\n", subscription.Spec.Subscriber, subscription.Spec.Stream)
		},
		UnsubscribeFunc: func(subscription eventingv1alpha1.Subscription) {
			fmt.Printf("Unubscribe %q from %q stream\n", subscription.Spec.Subscriber, subscription.Spec.Stream)
		},
	})

	m := createServer(monitor)
	m.Run()
}

func createServer(monitor *subscription.Monitor) *martini.ClassicMartini {
	m := martini.Classic()

	m.Post("/", func(req *http.Request, res http.ResponseWriter) {
		host := req.Host
		fmt.Printf("Recieved request for %s\n", host)
		stream, namespace := splitStreamName(host)
		subscriptions := monitor.Subscriptions(stream, namespace)
		if subscriptions == nil {
			res.WriteHeader(http.StatusNotFound)
			return
		}

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		res.WriteHeader(http.StatusAccepted)
		go func() {
			if len(*subscriptions) == 0 {
				fmt.Printf("No subscribers for stream %q\n", stream)
			}

			// make upstream requests
			client := &http.Client{}

			for _, subscription := range *subscriptions {
				go func(subscriber string) {
					fmt.Printf("Sending to %q for %q\n", subscriber, stream)

					url := fmt.Sprintf("http://%s/", subscriber)
					request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
					if err != nil {
						fmt.Printf("Unable to create subscriber request %v", err)
					}
					request.Header.Set("x-broker", broker)
					request.Header.Set("x-stream", stream)
					for _, header := range forwardHeaders {
						if value := req.Header.Get(header); value != "" {
							request.Header.Set(header, value)
						}
					}
					_, err = client.Do(request)
					if err != nil {
						fmt.Printf("Unable to complete subscriber request %v", err)
					}
				}(subscription.Subscriber)
			}
		}()
	})

	return m
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
