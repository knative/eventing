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
	"regexp"

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
	deployment   = "deployment"
	subscription = "subscription"
)

var (
	resourceFormat = regexp.MustCompile("^projects/([^/]+)/topics/([^/]+)$")
)

type GCPPubSubEventSource struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	image         string
	// namespace where the feed is created
	feedNamespace string
	// serviceAccount that the container runs as. Launches Receive Adapter with the
	// same Service Account
	feedServiceAccountName string
}

// The Go client libraries make you explicitly reconstruct the URI through builder patterns, so we sadly must crack it first.
func crackTopic(topic string) (projectID, topicID string, err error) {
	matches := resourceFormat.FindStringSubmatch(topic)
	if len(matches) != 3 {
		return "", "", fmt.Errorf(`Google Cloud Pub/Sub topics must match the pattern projects/*/topics/*";Got %q`, topic)
	}
	return matches[1], matches[2], nil
}

func NewGCPPubSubEventSource(kubeclientset kubernetes.Interface, feedNamespace string, feedServiceAccountName string, image string) sources.EventSource {
	glog.Infof("Image: %q", image)
	return &GCPPubSubEventSource{kubeclientset: kubeclientset, feedNamespace: feedNamespace, feedServiceAccountName: feedServiceAccountName, image: image}
}

func (t *GCPPubSubEventSource) StopFeed(trigger sources.EventTrigger, feedContext sources.FeedContext) error {
	glog.Infof("Stopping GCP PUBSUB Feed with context %+v", feedContext)

	projectID, _, err := crackTopic(trigger.Resource)
	if err != nil {
		glog.Warningf("Failed to delete deployment: %s", err)
		return err
	}

	if _, ok := feedContext.Context[deployment]; ok {
		deploymentName := feedContext.Context[deployment].(string)

		err = t.deleteWatcher(t.feedNamespace, deploymentName)
		if err != nil {
			glog.Warningf("Failed to delete deployment: %s", err)
			return err
		}
	}

	if _, ok := feedContext.Context[subscription]; ok {
		subscriptionName := feedContext.Context[subscription].(string)
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
	}
	return nil
}

func (t *GCPPubSubEventSource) StartFeed(trigger sources.EventTrigger, route string) (*sources.FeedContext, error) {
	glog.Infof("CREATING GCP PUBSUB feed")

	projectID, topicID, err := crackTopic(trigger.Resource)
	if err != nil {
		glog.Infof("Failed to create deployment: %v", err)
		return nil, err
	}

	// Just generate a random UUID as a subscriptionName
	uuid, err := uuid.NewRandom()
	if err != nil {
		glog.Infof("Failed to get create random uuid: %v", err)
		return nil, err
	}
	subscriptionName := fmt.Sprintf("sub-%s", uuid.String())

	glog.Infof("ProjectID: %q Topic: %q SubscriptionName: %q Route: %s", projectID, topicID, subscriptionName, route)

	ctx := context.Background()

	// TODO: Create a unique credential here just for this watcher...

	// Creates a client.
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		glog.Infof("Failed to create client: %v", err)
		return nil, err
	}

	sub, err := client.CreateSubscription(ctx, subscriptionName,
		pubsub.SubscriptionConfig{Topic: client.Topic(topicID)})
	if err != nil {
		glog.Infof("Failed to create subscription (might already exist): %+v", err)
		return nil, err
	}

	glog.Infof("Created subscription: %v", sub)

	// Create actual watcher
	deploymentName := subscriptionName
	err = t.createWatcher(deploymentName, t.image, projectID, topicID, subscriptionName, route)
	if err != nil {
		glog.Infof("Failed to create deployment: %v", err)

		// delete the subscription so it's not left floating around.
		errDelete := sub.Delete(ctx)
		if errDelete != nil {
			glog.Infof("Failed to delete subscription while trying to clean up %q : %s", subscriptionName, errDelete)
		}

		return nil, err
	}

	return &sources.FeedContext{
		Context: map[string]interface{}{
			subscription: subscriptionName,
			deployment:   deploymentName,
		}}, nil

}

func (t *GCPPubSubEventSource) createWatcher(deploymentName string, image string, projectID string, topicID string, subscription string, route string) error {
	dc := t.kubeclientset.AppsV1().Deployments(t.feedNamespace)

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

	// TODO: Create ownerref to the feed so when the feed goes away deployment
	// gets removed. Currently we manually delete the deployment.
	deployment := MakeWatcherDeployment(t.feedNamespace, deploymentName, t.feedServiceAccountName, image, projectID, topicID, subscription, route)
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

	sources.RunEventSource(NewGCPPubSubEventSource(kubeClient, feedNamespace, feedServiceAccountName, p.Image))
	log.Printf("Done...")
}
