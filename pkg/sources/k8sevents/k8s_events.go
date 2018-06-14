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
}

func NewK8SEventsEventSource(kubeclientset kubernetes.Interface, image string) sources.EventSource {
	glog.Infof("Image: %q", image)
	return &K8SEventsEventSource{kubeclientset: kubeclientset, image: image}
}

func (t *K8SEventsEventSource) Unbind(trigger sources.EventTrigger, bindContext sources.BindContext) error {
	glog.Infof("Unbinding K8S Events with context %+v", bindContext)

	deploymentName := bindContext.Context[deployment].(string)

	err := t.deleteWatcher("knative-eventing-system", deploymentName)
	if err != nil {
		glog.Warningf("Failed to delete deployment: %s", err)
		return err
	}
	return nil
}

func (t *K8SEventsEventSource) Bind(trigger sources.EventTrigger, route string) (*sources.BindContext, error) {
	glog.Infof("CREATING K8S Event binding")

	namespace := trigger.Parameters["namespace"].(string)

	// Just generate a random UUID as a subscriptionName
	uuid, err := uuid.NewRandom()
	subscriptionName := fmt.Sprintf("sub-%s", uuid.String())

	glog.Infof("Namespace: %q Route: %s", namespace, route)

	// Create actual watcher
	deploymentName := subscriptionName
	err = t.createWatcher("knative-eventing-system", deploymentName, t.image, namespace, route)
	if err != nil {
		glog.Infof("Failed to create deployment: %v", err)
		return nil, err
	}

	return &sources.BindContext{
		Context: map[string]interface{}{
			subscription: subscriptionName,
			deployment:   deploymentName,
		}}, nil

}

func (t *K8SEventsEventSource) createWatcher(namespace string, deploymentName string, image string, eventNamespace string, route string) error {
	dc := t.kubeclientset.AppsV1().Deployments(namespace)

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

	// TODO: Create ownerref to the binding so when the binding goes away deployment
	// gets removed. Currently we manually delete the deployment.
	deployment := MakeWatcherDeployment(namespace, deploymentName, "bind-controller", image, eventNamespace, route)
	_, createErr := dc.Create(deployment)
	return createErr
}

func (t *K8SEventsEventSource) deleteWatcher(namespace string, deploymentName string) error {
	dc := t.kubeclientset.AppsV1().Deployments(namespace)

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

	var p parameters
	err := json.Unmarshal(decodedParameters, &p)
	if err != nil {
		panic(fmt.Sprintf("can not unmarshal %q : %v", decodedParameters, err))
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %v", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building kubernetes clientset: %v", err.Error())
	}

	sources.RunEventSource(NewK8SEventsEventSource(kubeClient, p.Image))
	log.Printf("Done...")
}
