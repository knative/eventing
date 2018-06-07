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
	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	clientset "github.com/knative/eventing/pkg/client/clientset/versioned"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"github.com/knative/eventing/pkg/signals"
	"github.com/knative/eventing/pkg/subscription"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	masterURL  string
	kubeconfig string

	bus            = os.Getenv("BUS_NAME")
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

func splitChannelName(host string) (string, string) {
	chunks := strings.Split(host, ".")
	channel := chunks[0]
	namespace := chunks[1]
	return channel, namespace
}

func main() {
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	client, err := clientset.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building clientset: %s", err.Error())
	}

	informerFactory := informers.NewSharedInformerFactory(client, time.Second*30)
	monitor := subscription.NewMonitor(bus, informerFactory, subscription.MonitorEventHandlerFuncs{
		ProvisionFunc: func(channel channelsv1alpha1.Channel) {
			glog.Infof("Provision channel %q\n", channel.Name)
		},
		UnprovisionFunc: func(channel channelsv1alpha1.Channel) {
			glog.Infof("Unprovision channel %q\n", channel.Name)
		},
		SubscribeFunc: func(subscription channelsv1alpha1.Subscription) {
			glog.Infof("Subscribe %q to %q channel\n", subscription.Spec.Subscriber, subscription.Spec.Channel)
		},
		UnsubscribeFunc: func(subscription channelsv1alpha1.Subscription) {
			glog.Infof("Unubscribe %q from %q channel\n", subscription.Spec.Subscriber, subscription.Spec.Channel)
		},
	})
	go informerFactory.Start(stopCh)

	m := createServer(monitor)
	m.Run()

	glog.Flush()
}

func createServer(monitor *subscription.Monitor) *martini.ClassicMartini {
	m := martini.Classic()

	m.Post("/", func(req *http.Request, res http.ResponseWriter) {
		host := req.Host
		glog.Infof("Recieved request for %s\n", host)
		channel, namespace := splitChannelName(host)
		subscriptions := monitor.Subscriptions(channel, namespace)
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
				glog.Warningf("No subscribers for channel %q\n", channel)
			}

			// make upstream requests
			client := &http.Client{}

			for _, subscription := range *subscriptions {
				go func(subscriber string) {
					glog.Infof("Sending to %q for %q\n", subscriber, channel)

					url := fmt.Sprintf("http://%s/", subscriber)
					request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
					if err != nil {
						glog.Errorf("Unable to create subscriber request %v", err)
					}
					request.Header.Set("x-bus", bus)
					request.Header.Set("x-channel", channel)
					for _, header := range forwardHeaders {
						if value := req.Header.Get(header); value != "" {
							request.Header.Set(header, value)
						}
					}
					_, err = client.Do(request)
					if err != nil {
						glog.Errorf("Unable to complete subscriber request %v", err)
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
