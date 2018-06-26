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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"cloud.google.com/go/pubsub"
	"golang.org/x/net/context"
)

const (
	projectIDKey = "projectID"
	deployment   = "deployment"
	subscription = "subscription"
)

type GCPPubSubEventSource struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	image         string
	// namespace where the binding is created
	bindNamespace string
	// serviceAccount that the container runs as. Launches Receive Adapter with the
	// same Service Account
	bindServiceAccountName string
}

func NewGCPPubSubEventSource(kubeclientset kubernetes.Interface, bindNamespace string, bindServiceAccountName string, image string) sources.EventSource {
	glog.Infof("Image: %q", image)
	return &GCPPubSubEventSource{kubeclientset: kubeclientset, bindNamespace: bindNamespace, bindServiceAccountName: bindServiceAccountName, image: image}
}

func (t *GCPPubSubEventSource) Unbind(trigger sources.EventTrigger, bindContext sources.BindContext) error {
	glog.Infof("Unbinding GCP PUBSUB with context %+v", bindContext)

	projectID := trigger.Parameters[projectIDKey].(string)

	deploymentName := bindContext.Context[deployment].(string)
	subscriptionName := bindContext.Context[subscription].(string)

	err := t.deleteWatcher(t.bindNamespace, deploymentName)
	if err != nil {
		glog.Warningf("Failed to delete deployment: %s", err)
		return err
	}

	ctx := context.Background()
	// Creates a client.
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		glog.Infof("Failed to create client: %v", err)
		return err
	}

	sub := client.Subscription(subscriptionName)
	err = sub.Delete(ctx)
	if err != nil {
		glog.Warningf("Failed to delete subscription %q : %s : %+v", subscriptionName, err, err)
		// Return error only if it's something else than NotFound
		rpcStatus := status.Convert(err)
		if rpcStatus.Code() != codes.NotFound {
			return err
		}
		glog.Infof("Subscription %q already deleted", subscriptionName)
	}
	return nil
}

func (t *GCPPubSubEventSource) Bind(trigger sources.EventTrigger, route string) (*sources.BindContext, error) {
	glog.Infof("CREATING GCP PUBSUB binding")

	projectID := trigger.Parameters[projectIDKey].(string)
	topic := trigger.Parameters["topic"].(string)

	// Just generate a random UUID as a subscriptionName
	uuid, err := uuid.NewRandom()
	subscriptionName := fmt.Sprintf("sub-%s", uuid.String())

	glog.Infof("ProjectID: %q Topic: %q SubscriptionName: %q Route: %s", projectID, topic, subscriptionName, route)

	ctx := context.Background()

	// TODO: Create a unique credential here just for this watcher...

	// Creates a client.
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		glog.Infof("Failed to create client: %v", err)
		return nil, err
	}

	sub, err := client.CreateSubscription(ctx, subscriptionName,
		pubsub.SubscriptionConfig{Topic: client.Topic(topic)})
	if err != nil {
		glog.Infof("Failed to create subscription (might already exist): %+v", err)
		return nil, err
	}

	glog.Infof("Created subscription: %v", sub)

	// Create actual watcher
	deploymentName := subscriptionName
	err = t.createWatcher(deploymentName, t.image, projectID, subscriptionName, route)
	if err != nil {
		glog.Infof("Failed to create deployment: %v", err)

		// delete the subscription so it's not left floating around.
		errDelete := sub.Delete(ctx)
		if errDelete != nil {
			glog.Infof("Failed to delete subscription while trying to clean up %q : %s", subscriptionName, errDelete)
		}

		return nil, err
	}

	return &sources.BindContext{
		Context: map[string]interface{}{
			subscription: subscriptionName,
			deployment:   deploymentName,
		}}, nil

}

func (t *GCPPubSubEventSource) createWatcher(deploymentName string, image string, projectID string, subscription string, route string) error {
	dc := t.kubeclientset.AppsV1().Deployments(t.bindNamespace)

	// First, check if deployment exists already.
	if _, err := dc.Get(deploymentName, metav1.GetOptions{}); err != nil {
		if !apierrs.IsNotFound(err) {
			glog.Infof("deployments.Get for %q failed: %s", deploymentName, err)
			return err
		}
		glog.Infof("Deployment %q doesn't exist, creating", deploymentName)
	} else {
		glog.Infof("Found existing deployment %q", deploymentName)
		return nil
	}

	// TODO: Create ownerref to the binding so when the binding goes away deployment
	// gets removed. Currently we manually delete the deployment.
	deployment := MakeWatcherDeployment(t.bindNamespace, deploymentName, t.bindServiceAccountName, image, projectID, subscription, route)
	_, createErr := dc.Create(deployment)
	return createErr
}

func (t *GCPPubSubEventSource) deleteWatcher(namespace string, deploymentName string) error {
	dc := t.kubeclientset.AppsV1().Deployments(namespace)

	// First, check if deployment exists already.
	if _, err := dc.Get(deploymentName, metav1.GetOptions{}); err != nil {
		if !apierrs.IsNotFound(err) {
			glog.Infof("deployments.Get for %q failed: %s", deploymentName, err)
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

	sources.RunEventSource(NewGCPPubSubEventSource(kubeClient, bindNamespace, bindServiceAccountName, p.Image))
	log.Printf("Done...")
}
