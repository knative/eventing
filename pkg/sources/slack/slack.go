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
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/golang/glog"
	"github.com/knative/eventing/pkg/sources"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"

	servingclientset "github.com/knative/serving/pkg/client/clientset/versioned"

	"github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"strings"
)

const (
	// postfixReceiveAdapter is appended to the name of the service running the Receive Adapter
	postfixReceiveAdapter = "rcvadptr"

	// secretName is the name of the secret that contains the Slack credentials.
	secretName = "secretName"
	// secretKey is the name of the key inside the secret that contains the Slack credentials.
	secretKey = "secretKey"
)

type SlackEventSource struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset    kubernetes.Interface
	servingclientset servingclientset.Interface
	image            string
	// namespace where the feed is created
	feedNamespace string
	// serviceAccount that the container runs as. Launches Receive Adapter with the
	// same Service Account
	feedServiceAccountName string
}

func NewSlackEventSource(kubeclientset kubernetes.Interface, servingclientset servingclientset.Interface, feedNamespace string, feedServiceAccountName string, image string) sources.EventSource {
	glog.Infof("Creating slack event source.")
	return &SlackEventSource{kubeclientset: kubeclientset, servingclientset: servingclientset, feedNamespace: feedNamespace, feedServiceAccountName: feedServiceAccountName, image: image}
}

func (t *SlackEventSource) StopFeed(trigger sources.EventTrigger, feedContext sources.FeedContext) error {
	glog.Infof("Stopping Slack feed with context %+v", feedContext)

	serviceName := makeServiceName(trigger.Resource)
	err := t.deleteReceiveAdapter(serviceName)
	if err != nil {
		glog.Infof("Unable to delete Slack Knative service: %v", err)
	}

	return err
}

func (t *SlackEventSource) StartFeed(trigger sources.EventTrigger, target string) (*sources.FeedContext, error) {
	glog.Infof("Creating Slack feed.")

	service, err := t.createReceiveAdapter(trigger, target)
	if err != nil {
		glog.Warningf("Failed to create slack service: %v", err)
		return nil, err
	}

	glog.Infof("Created Slack service: %+v", service)

	return &sources.FeedContext{
		Context: map[string]interface{}{}}, nil
}

func makeServiceName(resource string) string {
	serviceName := fmt.Sprintf("%s-%s-%s", "slack", resource, postfixReceiveAdapter) // TODO: this needs more UUID on the end of it.
	serviceName = strings.ToLower(serviceName)
	return serviceName
}

func (t *SlackEventSource) createReceiveAdapter(trigger sources.EventTrigger, target string) (*v1alpha1.Service, error) {
	sc := t.servingclientset.ServingV1alpha1().Services(t.feedNamespace)

	serviceName := makeServiceName(trigger.Resource)

	// First, check if the service already exists.
	if svc, err := sc.Get(serviceName, metav1.GetOptions{}); err != nil {
		if !apierrs.IsNotFound(err) {
			glog.Infof("Knative service.Get for %q failed: %s", serviceName, err)
			return nil, err
		}
		glog.Infof("Knative service %q doesn't exist, creating", serviceName)
	} else {
		glog.Infof("Found existing Knative service %q", serviceName)
		return svc, nil
	}

	service := MakeService(t.feedNamespace, serviceName, t.feedServiceAccountName, t.image, target, trigger.Parameters[secretName].(string), trigger.Parameters[secretKey].(string))
	svc, createErr := sc.Create(service)
	if createErr != nil {
		glog.Errorf("Knative serving failed: %s", createErr)
	}
	return svc, createErr
}

func (t *SlackEventSource) deleteReceiveAdapter(serviceName string) error {
	sc := t.servingclientset.ServingV1alpha1().Services(t.feedNamespace)

	// First, check if the Knative service actually exists.
	if _, err := sc.Get(serviceName, metav1.GetOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			glog.Infof("Knative serving %q already deleted", serviceName)
			return nil
		}
		glog.Infof("Knative Services.Get for %q failed: %v", serviceName, err)
		return err
	}

	return sc.Delete(serviceName, &metav1.DeleteOptions{})
}

type parameters struct {
	Image string `json:"image,omitempty"`
}

func main() {
	flag.Parse()

	flag.Lookup("logtostderr").Value.Set("true")

	log.Printf("Starting up the main slack!")

	decodedParameters, _ := base64.StdEncoding.DecodeString(os.Getenv(sources.EventSourceParametersKey))

	feedNamespace := os.Getenv(sources.FeedNamespaceKey)
	feedServiceAccountName := os.Getenv(sources.FeedServiceAccountKey)

	var p parameters
	err := json.Unmarshal(decodedParameters, &p)
	if err != nil {
		panic(fmt.Sprintf("can not unmarshal %q : %s", decodedParameters, err))
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	servingClient, err := servingclientset.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("error building serving clientset: %s", err.Error())
	}

	sources.RunEventSource(NewSlackEventSource(kubeClient, servingClient, feedNamespace, feedServiceAccountName, p.Image))
	log.Printf("Done...")
}
