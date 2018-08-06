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
	deployment = "deployment"
)

type AlarmEventSource struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	image         string
	// namespace where the binding is created
	bindNamespace string
	// serviceAccount that the container runs as. Launches Receive Adapter with the
	// same Service Account
	bindServiceAccountName string
}

func NewAlarmEventSource(kubeclientset kubernetes.Interface, bindNamespace string, bindServiceAccountName string, image string) sources.EventSource {
	glog.Infof("Image: %q", image)
	return &AlarmEventSource{kubeclientset: kubeclientset, bindNamespace: bindNamespace, bindServiceAccountName: bindServiceAccountName, image: image}
}

func (a *AlarmEventSource) Unbind(trigger sources.EventTrigger, bindContext sources.BindContext) error {
	glog.Infof("Unbinding alarm with context %+v", bindContext)
	// delete timer deployment
	deploymentName := bindContext.Context[deployment].(string)

	err := a.deleteAlarmDeployment(deploymentName)
	if err != nil {
		glog.Warningf("Failed to delete deployment: %s", err)
		return err
	}
	return nil
}

func (a *AlarmEventSource) Bind(trigger sources.EventTrigger, route string) (*sources.BindContext, error) {
	glog.Infof("bind alarm event")
	// create timer deployment
	interval := trigger.Parameters["interval"].(string)
	until := trigger.Parameters["until"].(string)
	glog.Infof("interval %q, until %q, route %q", interval, until, route)

	// create a random deployment name
	uuid, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	deploymentName := "alarm-" + uuid.String()
	err = a.createAlarmDeployment(deployment, a.image, interval, until, route)
	if err != nil {
		glog.Warningf("Failed to create deployment: %s", err)
		return nil, err
	}
	return &sources.BindContext{
		Context: map[string]interface{}{
			deployment: deploymentName,
		}}, nil
}

func (a *AlarmEventSource) createAlarmDeployment(deploymentName, image, interval, until, route string) error {
	dc := a.kubeclientset.AppsV1().Deployments(a.bindNamespace)

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
	deployment := MakeAlarmDeployment(a.bindNamespace, deploymentName, image, interval, until, route)
	_, createErr := dc.Create(deployment)
	return createErr
}

func (a *AlarmEventSource) deleteAlarmDeployment(deploymentName string) error {
	dc := a.kubeclientset.AppsV1().Deployments(a.bindNamespace)

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

	bindNamespace := os.Getenv(sources.BindNamespaceKey)
	bindServiceAccountName := os.Getenv(sources.BindServiceAccountKey)

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

	sources.RunEventSource(NewAlarmEventSource(kubeClient, bindNamespace, bindServiceAccountName, p.Image))
	log.Printf("Done...")
}
