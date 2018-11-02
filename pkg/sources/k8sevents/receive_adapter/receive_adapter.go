/*
Copyright 2018 The Knative Authors

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
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/golang/glog"
	"github.com/knative/pkg/cloudevents"
	"github.com/knative/pkg/signals"
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
		log.Fatalf("Error building kubeconfig: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("Error building kubernetes clientset: %v", err)
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

// Creates a URI of the form found in object metadata selfLinks
// Format looks like: /apis/feeds.knative.dev/v1alpha1/namespaces/default/feeds/k8s-events-example
// KNOWN ISSUES:
// * ObjectReference.APIVersion has no version information (e.g. serving.knative.dev rather than serving.knative.dev/v1alpha1)
// * ObjectReference does not have enough information to create the pluaralized list type (e.g. "revisions" from kind: Revision)
//
// Track these issues at https://github.com/kubernetes/kubernetes/issues/66313
// We could possibly work around this by adding a lister for the resources referenced by these events.
func createSelfLink(o corev1.ObjectReference) string {
	collectionNameHack := strings.ToLower(o.Kind) + "s"
	versionNameHack := o.APIVersion

	// Core API types don't have a separate package name and only have a version string (e.g. /apis/v1/namespaces/default/pods/myPod)
	// To avoid weird looking strings like "v1/versionUnknown" we'll sniff for a "." in the version
	if strings.Contains(versionNameHack, ".") && !strings.Contains(versionNameHack, "/") {
		versionNameHack = versionNameHack + "/versionUnknown"
	}
	return fmt.Sprintf("/apis/%s/namespaces/%s/%s/%s", versionNameHack, o.Namespace, collectionNameHack, o.Name)
}

// Creates a CloudEvent Context for a given K8S event. For clarity, the following is a spew-dump
// of a real K8S event:
//	&Event{
//		ObjectMeta:k8s_io_apimachinery_pkg_apis_meta_v1.ObjectMeta{
//			Name:k8s-events-00001.1542495ae3f5fbba,
//			Namespace:default,
//			SelfLink:/api/v1/namespaces/default/events/k8s-events-00001.1542495ae3f5fbba,
//			UID:fc2fffb3-8a12-11e8-8874-42010a8a0fd9,
//			ResourceVersion:2729,
//			Generation:0,
//			CreationTimestamp:2018-07-17 22:44:37 +0000 UTC,
//		},
//		InvolvedObject:ObjectReference{
//			Kind:Revision,
//			Namespace:default,
//			Name:k8s-events-00001,
//			UID:f5c19306-8a12-11e8-8874-42010a8a0fd9,
//			APIVersion:serving.knative.dev,
//			ResourceVersion:42683,
//		},
//		Reason:RevisionReady,
//		Message:Revision becomes ready upon endpoint "k8s-events-00001-service" becoming ready,
//		Source:EventSource{
//			Component:revision-controller,
//		},
//		FirstTimestamp:2018-07-17 22:44:37 +0000 UTC,
//		LastTimestamp:2018-07-17 22:49:40 +0000 UTC,
//		Count:91,
//		Type:Normal,
//		EventTime:0001-01-01 00:00:00 +0000 UTC,
//	}
func cloudEventsContext(m *corev1.Event) *cloudevents.EventContext {
	return &cloudevents.EventContext{
		// Events are themselves object and have a unique UUID. Could also have used the UID
		CloudEventsVersion: cloudevents.CloudEventsVersion,
		EventType:          "dev.knative.k8s.event",
		EventID:            string(m.ObjectMeta.UID),
		Source:             createSelfLink(m.InvolvedObject),
		EventTime:          m.ObjectMeta.CreationTimestamp.Time,
	}
}

func postMessage(target string, m *corev1.Event) error {
	ctx := cloudEventsContext(m)

	URL := fmt.Sprintf("http://%s/", target)
	log.Printf("Posting to %q", URL)
	// Explicitly using Binary encoding so that Istio, et. al. can better inspect
	// event metadata.
	req, err := cloudevents.Binary.NewRequest(URL, m, *ctx)
	if err != nil {
		log.Printf("Failed to create http request: %s", err)
		return err
	}

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
