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
	"log"
	"os"

	"github.com/golang/glog"
	"github.com/google/uuid"
	"github.com/knative/eventing/pkg/sources"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	projectIDKey = "projectID"
	deployment   = "deployment"
	subscription = "subscription"
)

type K8SEventsEventSource struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	image         string
	// namespace where the feed is created
	feedNamespace string
	// serviceAccount that the container runs as. Launches Receive Adapter with the
	// same Service Account
	feedServiceAccountName string
}

func NewK8SEventsEventSource(kubeclientset kubernetes.Interface, feedNamespace string, feedServiceAccountName string, image string) sources.EventSource {
	glog.Infof("Image: %q", image)
	return &K8SEventsEventSource{kubeclientset: kubeclientset, feedNamespace: feedNamespace, feedServiceAccountName: feedServiceAccountName, image: image}
}

func (t *K8SEventsEventSource) StopFeed(trigger sources.EventTrigger, feedContext sources.FeedContext) error {
	glog.Infof("Stopping K8S Events feed with context %+v", feedContext)

	deploymentName := feedContext.Context[deployment].(string)

	err := t.deleteWatcher(deploymentName)
	if err != nil {
		glog.Warningf("Failed to delete deployment: %s", err)
		return err
	}
	return nil
}

func (t *K8SEventsEventSource) StartFeed(trigger sources.EventTrigger, route string) (*sources.FeedContext, error) {
	glog.Infof("CREATING K8S Event feed")

	namespace := trigger.Parameters["namespace"].(string)

	// Just generate a random UUID as a subscriptionName
	uuid, err := uuid.NewRandom()
	if err != nil {
		glog.Infof("Failed to create new random uuid: %v", err)
		return nil, err
	}
	subscriptionName := fmt.Sprintf("sub-%s", uuid.String())

	glog.Infof("Namespace: %q Route: %s", namespace, route)

	// Create actual watcher
	deploymentName := subscriptionName
	err = t.createWatcher(deploymentName, t.image, namespace, route)
	if err != nil {
		glog.Infof("Failed to create deployment: %v", err)
		return nil, err
	}

	return &sources.FeedContext{
		Context: map[string]interface{}{
			subscription: subscriptionName,
			deployment:   deploymentName,
		}}, nil

}

func (t *K8SEventsEventSource) createWatcher(deploymentName string, image string, eventNamespace string, route string) error {
	dc := t.kubeclientset.AppsV1().Deployments(t.feedNamespace)

	// First, check if deployment exists already.
	if _, err := dc.Get(deploymentName, metav1.GetOptions{}); err != nil {
		if !apierrs.IsNotFound(err) {
			glog.Infof("deployments.Get for %q failed: %v", deploymentName, err)
			return err
		}
		glog.Infof("Deployment %q doesn't exist, creating", deploymentName)
	} else {
		glog.Infof("Found existing deployment %q", deploymentName)
		return nil
	}

	// TODO: Create ownerref to the feed so when the feed goes away deployment
	// gets removed. Currently we manually delete the deployment.
	deployment := MakeWatcherDeployment(t.feedNamespace, deploymentName, t.feedServiceAccountName, image, eventNamespace, route)
	_, createErr := dc.Create(deployment)
	return createErr
}

func (t *K8SEventsEventSource) deleteWatcher(deploymentName string) error {
	dc := t.kubeclientset.AppsV1().Deployments(t.feedNamespace)

	// First, check if deployment exists already.
	if _, err := dc.Get(deploymentName, metav1.GetOptions{}); err != nil {
		if !apierrs.IsNotFound(err) {
			glog.Infof("deployments.Get for %q failed: %v", deploymentName, err)
			return err
		}
		glog.Infof("Deployment %q already deleted", deploymentName)
		return nil
	}

	return dc.Delete(deploymentName, &metav1.DeleteOptions{})
}

type parameters struct {
	Image string `json:"image,omitempty"`
}

func main() {
	flag.Parse()

	decodedParameters, _ := base64.StdEncoding.DecodeString(os.Getenv(sources.EventSourceParametersKey))

	feedNamespace := os.Getenv(sources.FeedNamespaceKey)
	feedServiceAccountName := os.Getenv(sources.FeedServiceAccountKey)

	var p parameters
	err := json.Unmarshal(decodedParameters, &p)
	if err != nil {
		panic(fmt.Sprintf("can not unmarshal %q : %v", decodedParameters, err))
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building kubernetes clientset: %v", err)
	}

	sources.RunEventSource(NewK8SEventsEventSource(kubeClient, feedNamespace, feedServiceAccountName, p.Image))
	log.Printf("Done...")
}
