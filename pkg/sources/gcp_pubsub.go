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

package sources

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/google/uuid"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"cloud.google.com/go/pubsub"
	"golang.org/x/net/context"
)

const (
	projectIDKey = "projectID"
	deployment   = "deployment"
	subscription = "subscription"
)

// TODO: This and the github example need to move out of proc so they can be invoked
// either with a webhook or by invoking a container. Regardless, that's a bigger
// refactor and hence a followup.
type GCPPubSubEventSource struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	image         string
}

func NewGCPPubSubEventSource(kubeclientset kubernetes.Interface, image string) EventSource {
	glog.Infof("Image: %q", image)
	return &GCPPubSubEventSource{kubeclientset: kubeclientset, image: image}
}

func (t *GCPPubSubEventSource) Unbind(trigger EventTrigger, bindContext BindContext) error {
	glog.Infof("Unbinding GCP PUBSUB with context %+v", bindContext)

	projectID := trigger.Parameters[projectIDKey].(string)

	deploymentName := bindContext.Context[deployment].(string)
	subscriptionName := bindContext.Context[subscription].(string)

	err := t.deleteWatcher("bind-system", deploymentName)
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
		glog.Warningf("Failed to delete subscription %q : %s", subscriptionName, err)
		return err
	}
	return nil
}

func (t *GCPPubSubEventSource) Bind(trigger EventTrigger, route string) (*BindContext, error) {
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
	err = t.createWatcher("bind-system", deploymentName, t.image, projectID, subscriptionName, route)
	if err != nil {
		glog.Infof("Failed to create deployment: %v", err)
		return nil, err
	}

	return &BindContext{
		Context: map[string]interface{}{
			subscription: subscriptionName,
			deployment:   deploymentName,
		}}, nil

}

func (t *GCPPubSubEventSource) createWatcher(namespace string, deploymentName string, image string, projectID string, subscription string, route string) error {
	dc := t.kubeclientset.AppsV1().Deployments(namespace)

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
	deployment := MakeWatcherDeployment(namespace, deploymentName, image, projectID, subscription, route)
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
