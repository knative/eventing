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
	"github.com/knative/eventing/pkg/sources"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"

	servingclientset "github.com/knative/serving/pkg/client/clientset/versioned"

	"github.com/knative/eventing/pkg/sources/slack/resources"
	"github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"strings"
	)

const (
	// receiveAdapterSuffix is appended to the name of the service running the Receive Adapter
	receiveAdapterSuffix = "rcvadptr"

	// secretNameKey is the name of the secret that contains the Slack credentials.
	secretNameKey = "secretName"
	// secretKeyKey is the name of the key inside the secret that contains the Slack credentials.
	secretKeyKey = "secretKey"
)

type SlackEventSource struct {
	// kubeclientset is a standard kubernetes clientset.
	kubeclientset kubernetes.Interface
	// servingclientset is a standard serving.knative.dev clientset.
	servingclientset servingclientset.Interface
	// image is the container image used as the receive adapter.
	image string
	// namespace where the feed is created.
	feedNamespace string
	// serviceAccount that the container runs as. Launches Receive Adapter with the
	// same Service Account.
	feedServiceAccountName string
}

func NewSlackEventSource(kubeclientset kubernetes.Interface, servingclientset servingclientset.Interface, feedNamespace string, feedServiceAccountName string, image string) sources.EventSource {
	log.Printf("Creating slack event source.")
	return &SlackEventSource{
		kubeclientset:          kubeclientset,
		servingclientset:       servingclientset,
		feedNamespace:          feedNamespace,
		feedServiceAccountName: feedServiceAccountName,
		image:                  image,
	}
}

func (t *SlackEventSource) StopFeed(trigger sources.EventTrigger, feedContext sources.FeedContext) error {
	log.Printf("Stopping Slack feed with context %+v", feedContext)

	serviceName := makeServiceName(trigger.Resource)
	err := t.deleteReceiveAdapter(serviceName)
	if err != nil {
		log.Printf("Unable to delete Slack Knative service: %v", err)
	}

	return err
}

func (t *SlackEventSource) StartFeed(trigger sources.EventTrigger, target string) (*sources.FeedContext, error) {
	log.Printf("Creating Slack feed.")

	service, err := t.createReceiveAdapter(trigger, target)
	if err != nil {
		log.Printf("Failed to create Slack service: %v", err)
		return nil, err
	}

	log.Printf("Created Slack service: %+v", service)

	return &sources.FeedContext{
		Context: map[string]interface{}{}}, nil
}

func makeServiceName(resource string) string {
	serviceName := fmt.Sprintf("%s-%s-%s", "slack", resource, receiveAdapterSuffix) // TODO: this needs more UUID on the end of it.
	serviceName = strings.ToLower(serviceName)
	return serviceName
}

func (t *SlackEventSource) createReceiveAdapter(trigger sources.EventTrigger, target string) (*v1alpha1.Service, error) {
	sc := t.servingclientset.ServingV1alpha1().Services(t.feedNamespace)

	serviceName := makeServiceName(trigger.Resource)

	// First, check if the service already exists.
	if svc, err := sc.Get(serviceName, metav1.GetOptions{}); err != nil {
		if !apierrs.IsNotFound(err) {
			log.Printf("Knative service.Get for %q failed: %v", serviceName, err)
			return nil, err
		}
		log.Printf("Knative service %q doesn't exist, creating", serviceName)
	} else {
		log.Printf("Found existing Knative service %q", serviceName)
		return svc, nil
	}

	secretName, err := stringFrom(trigger.Parameters, secretNameKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret name from trigger parameters: %s, %v", secretNameKey, err)
	}

	secretKey, err := stringFrom(trigger.Parameters, secretKeyKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret key from trigger parameters: %s, %v", secretKeyKey, err)
	}

	service := resources.MakeService(t.feedNamespace, serviceName, t.feedServiceAccountName, t.image, target, secretName, secretKey)
	svc, createErr := sc.Create(service)
	if createErr != nil {
		log.Printf("failed to create Knative service: %s", createErr)
	}
	return svc, createErr
}

func (t *SlackEventSource) deleteReceiveAdapter(serviceName string) error {
	sc := t.servingclientset.ServingV1alpha1().Services(t.feedNamespace)

	// First, check if the Knative service actually exists.
	if _, err := sc.Get(serviceName, metav1.GetOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			log.Printf("Knative service %q already deleted", serviceName)
			return nil
		}
		log.Printf("Knative Services.Get for %q failed: %v", serviceName, err)
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
		log.Printf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Printf("Error building kubernetes clientset: %s", err.Error())
	}

	servingClient, err := servingclientset.NewForConfig(cfg)
	if err != nil {
		log.Printf("error building serving clientset: %s", err.Error())
	}

	sources.RunEventSource(NewSlackEventSource(kubeClient, servingClient, feedNamespace, feedServiceAccountName, p.Image))
	log.Printf("Done...")
}

// TODO(n3wscott): Move this to knative/pkg/context.
func stringFrom(bag map[string]interface{}, key string) (string, error) {
	// check to see if the key is in the bag.
	if _, ok := bag[key]; !ok {
		return "", fmt.Errorf("%s not found", key)
	}
	value, ok := bag[key].(string)
	if !ok {
		return "", fmt.Errorf("value for %s was not a valid string", key)
	}
	return value, nil
}
