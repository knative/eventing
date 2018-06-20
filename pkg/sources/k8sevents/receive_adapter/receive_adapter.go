/*
Copyright 2018 Google LLC

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
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/golang/glog"
	"github.com/knative/eventing/pkg/signals"
	corev1 "k8s.io/api/core/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// Target for messages
	envTarget = "TARGET"
	// Namespace to watch
	envNamespace = "NAMESPACE"
)

type EventWatcher struct {
	target string
}

func NewEventWatcher(target string) *EventWatcher {
	return &EventWatcher{target: target}
}

func (e *EventWatcher) updateEvent(old, new interface{}) {
	e.addEvent(new)
}

func (e *EventWatcher) addEvent(new interface{}) {
	event := new.(*corev1.Event)
	log.Printf("GOT EVENT: %+v", event)
	postMessage(e.target, event)
}

func main() {
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	target := os.Getenv(envTarget)
	namespace := os.Getenv(envNamespace)

	log.Printf("Target is: %q", target)
	log.Printf("Namespace is: %q", namespace)

	cfg, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("Error building kubernetes clientset: %v", err.Error())
	}

	log.Printf("Creating a new Event Watcher...")
	watcher := NewEventWatcher(target)

	eventsInformer := coreinformers.NewFilteredEventInformer(
		kubeClient, namespace, 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, nil)

	eventsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    watcher.addEvent,
		UpdateFunc: watcher.updateEvent,
	})

	log.Printf("Starting eventsInformer...")
	go eventsInformer.Run(stopCh)

	log.Printf("Waiting for caches to sync...")
	if ok := cache.WaitForCacheSync(stopCh, eventsInformer.HasSynced); !ok {
		glog.Fatalf("Failed to wait for events cache to sync")
	}
	log.Printf("Caches synced...")
	<-stopCh
	log.Printf("Exiting...")
}

func postMessage(target string, m *corev1.Event) error {
	jsonStr, err := json.Marshal(m)
	if err != nil {
		log.Printf("Failed to marshal the message: %+v : %s", m, err)
		return err
	}

	URL := fmt.Sprintf("http://%s/", target)
	log.Printf("Posting to %q", URL)
	req, err := http.NewRequest("POST", URL, bytes.NewBuffer(jsonStr))
	if err != nil {
		log.Printf("Failed to create http request: %s", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to do POST: %v", err)
		return err
	}
	defer resp.Body.Close()
	log.Printf("response Status: %s", resp.Status)
	body, _ := ioutil.ReadAll(resp.Body)
	log.Printf("response Body: %s", string(body))
	return nil
}
