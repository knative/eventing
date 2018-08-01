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
	"context"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/golang/glog"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/eventing/pkg/sources"
	servingclientset "github.com/knative/serving/pkg/client/clientset/versioned"
	"golang.org/x/oauth2"

	"encoding/base64"
	"encoding/json"
	"os"

	ghclient "github.com/google/go-github/github"
	"github.com/knative/eventing/pkg/sources/github/resources"
	"github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"time"
)

const (
	webhookIDKey = "id"
	ownerKey     = "owner"
	repoKey      = "repo"

	// SuccessSynced is used as part of the Event 'reason' when a Feed is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a Feed
	// is synced successfully
	MessageResourceSynced = "Feed synced successfully"

	// property bag keys
	accessTokenKey = "accessToken"
	secretTokenKey = "secretToken"

	// postfixReceiveAdapter is appended to the name of the service running the Receive Adapter
	postfixReceiveAdapter = "rcvadptr"

	// watchTimeout is the timeout that the feedlet will wait for the Receiver Adapter to get a domain name.
	watchTimeout = 5 * time.Minute

	// secretNameKey is the name of the secret that contains the GitHub credentials.
	secretNameKey = "secretName"
	// secretKeyKey is the name of key inside the secret that contains the GitHub credentials.
	secretKeyKey = "secretKey"
)

type GithubEventSource struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// servingclientset is a clientset for serving API groups
	servingclientset servingclientset.Interface
	// namespace where the feed is created
	feedNamespace string
	// serviceAccount that the container runs as. Launches Receive Adapter with the
	// same Service Account
	feedServiceAccountName string
	// image for the receive adapter
	image string
}

func NewGithubEventSource(kubeclientset kubernetes.Interface, servingclientset servingclientset.Interface, feedNamespace, feedServiceAccountName, image string) sources.EventSource {
	return &GithubEventSource{
		kubeclientset:          kubeclientset,
		servingclientset:       servingclientset,
		feedNamespace:          feedNamespace,
		feedServiceAccountName: feedServiceAccountName,
		image: image,
	}
}

// TODO(n3wscott): Add a timeout for StopFeed.
func (t *GithubEventSource) StopFeed(trigger sources.EventTrigger, feedContext sources.FeedContext) error {
	glog.Infof("stopping github webhook feed with context %+v", feedContext)

	serviceName := receiveAdapterName(trigger.Resource)
	t.deleteReceiveAdapter(t.feedNamespace, serviceName)

	components := strings.Split(trigger.Resource, "/")
	owner := components[0]
	repo := components[1]

	webhookID, err := stringFrom(feedContext.Context, webhookIDKey)
	if err != nil {
		glog.Errorf("Failed to get webhook id from context: %v; bailing...", err)
		return nil
	}

	accessToken, err := stringFrom(trigger.Parameters, accessTokenKey)
	if err != nil {
		glog.Errorf("Failed to get access token from trigger parameters: %v; bailing...", err)
		return nil
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := ghclient.NewClient(tc)

	id, err := strconv.ParseInt(webhookID, 10, 64)
	if err != nil {
		glog.Errorf("failed to convert webhook %q to int64 : %s", webhookID, err)
		return err
	}
	_, err = client.Repositories.DeleteHook(ctx, owner, repo, id)
	if err != nil {
		if errResp, ok := err.(*ghclient.ErrorResponse); ok && errResp.Message == "Not Found" {
			// If the webhook doesn't exist, nothing to do
			glog.Errorf("webhook doesn't exist, nothing to delete.")
			return nil
		}
		glog.Errorf("failed to delete the webhook: %v", err)
		return err
	}
	glog.Infof("deleted webhook %q successfully", webhookID)
	return nil
}

// TODO(n3wscott): Add a timeout for StartFeed.
func (t *GithubEventSource) StartFeed(trigger sources.EventTrigger, target string) (*sources.FeedContext, error) {

	// Create the Receive Adapter Service that will accept incoming requests from GitHub.
	service, err := t.createReceiveAdapter(trigger, target)
	if err != nil {
		glog.Warningf("failed to create service: %v", err)
		return nil, err
	}

	// TODO(n3wscott): look into using spew.
	glog.Infof("created Service: %+v", service)

	// Start watching the Receive Adapter Service for it's updated domain name. This will be passed
	// to GitHub as part of the webhook registration.
	receiveAdaptorDomain, err := t.waitForServiceDomain(service.GetObjectMeta().GetName())
	if err != nil {
		glog.Infof("failed to get the service: %v", err)
	}

	return t.createWebhook(trigger, receiveAdaptorDomain)
}

func (t *GithubEventSource) waitForServiceDomain(serviceName string) (string, error) {
	sc := t.servingclientset.ServingV1alpha1().Services(t.feedNamespace)

	wt := int64(watchTimeout / time.Second)
	opts := metav1.ListOptions{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1alpha1",
		},
		FieldSelector:  fmt.Sprintf("metadata.name=%s", serviceName),
		LabelSelector:  "receive-adapter=github",
		Watch:          true,
		TimeoutSeconds: &wt,
	}

	w, err := sc.Watch(opts)
	if err != nil {
		glog.Infof("failed to create watch: %v", err)
		return "", err
	}

	eventChan := w.ResultChan()
	for {
		event, more := <-eventChan
		if !more {
			glog.Infof("expected a Service object, but got %T", event.Object)
			return "", fmt.Errorf("no more")
		}
		switch event.Type {
		case watch.Error:
			return "", fmt.Errorf("watched Service error")
		case watch.Deleted:
			return "", fmt.Errorf("watched Service deleted")
		case watch.Added:
			fallthrough
		case watch.Modified:
			service, ok := event.Object.(*v1alpha1.Service)
			if !ok {
				glog.Infof("expected a Service object, but got %T", event.Object)
				continue
			}
			if service.Name == serviceName {
				status := service.Status
				if status.Domain != "" {
					w.Stop()
					glog.Infof("got domain: %s", event.Object)
					return status.Domain, nil
				}
			}
		}
	}
}

func receiveAdapterName(resource string) string {
	serviceName := fmt.Sprintf("%s-%s-%s", "github", resource, postfixReceiveAdapter) // TODO: this needs more UUID on the end of it.
	serviceName = strings.Replace(serviceName, "/", "-", -1)
	serviceName = strings.Replace(serviceName, ".", "-", -1)
	serviceName = strings.ToLower(serviceName)
	return serviceName
}

func (t *GithubEventSource) createReceiveAdapter(trigger sources.EventTrigger, target string) (*v1alpha1.Service, error) {
	sc := t.servingclientset.ServingV1alpha1().Services(t.feedNamespace)

	serviceName := receiveAdapterName(trigger.Resource)

	// First, check if service exists already.
	if _, err := sc.Get(serviceName, metav1.GetOptions{}); err != nil {
		if !apierrs.IsNotFound(err) {
			glog.Infof("service.Get for %q failed: %s", serviceName, err)
			return nil, err
		}
		glog.Infof("service %q doesn't exist, creating", serviceName)
	} else {
		glog.Infof("found existing service %q", serviceName)
		return nil, nil
	}

	secretName, err := stringFrom(trigger.Parameters, secretNameKey)
	if err != nil {
		return nil, fmt.Errorf("Failed to get secret name from trigger parameters: %v", err)
	}

	secretKey, err := stringFrom(trigger.Parameters, secretKeyKey)
	if err != nil {
		return nil, fmt.Errorf("Failed to get secret key from trigger parameters: %v", err)
	}

	service := resources.MakeService(t.feedNamespace, serviceName, t.image, t.feedServiceAccountName, target, secretName, secretKey)
	service, createErr := sc.Create(service)
	if createErr != nil {
		glog.Errorf("service failed: %s", createErr)
	}

	return service, createErr
}

func (t *GithubEventSource) deleteReceiveAdapter(namespace string, serviceName string) error {
	sc := t.servingclientset.ServingV1alpha1().Services(t.feedNamespace)

	// First, check if deployment exists already.
	if _, err := sc.Get(serviceName, metav1.GetOptions{}); err != nil {
		if !apierrs.IsNotFound(err) {
			glog.Infof("services.Get for %q failed: %s", serviceName, err)
			return err
		}
		glog.Infof("service %q already deleted", serviceName)
		return nil
	}

	return sc.Delete(serviceName, &metav1.DeleteOptions{})
}

func (t *GithubEventSource) deleteWebhook(trigger sources.EventTrigger, feedContext sources.FeedContext) error {
	glog.Infof("Stopping github webhook feed with context %+v", feedContext)

	components := strings.Split(trigger.Resource, "/")
	owner := components[0]
	repo := components[1]

	webhookID, err := stringFrom(feedContext.Context, webhookIDKey)
	if err != nil {
		return fmt.Errorf("Failed to get webhook ID from context: %v", err)
	}

	accessToken, err := stringFrom(trigger.Parameters, accessTokenKey)
	if err != nil {
		return fmt.Errorf("Failed to get access token from trigger parameters: %v", err)
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := ghclient.NewClient(tc)

	id, err := strconv.ParseInt(webhookID, 10, 64)
	if err != nil {
		glog.Warningf("Failed to convert webhook %q to int64 : %s", webhookID, err)
		return err
	}
	r, err := client.Repositories.DeleteHook(ctx, owner, repo, id)
	if err != nil {
		if errResp, ok := err.(*ghclient.ErrorResponse); ok {
			// If the webhook doesn't exist, nothing to do
			if errResp.Message == "Not Found" {
				glog.Info("Webhook doesn't exist, nothing to delete.")
				return nil
			}
		}
		glog.Warningf("Failed to delete the webhook: %s", err)
		glog.Warningf("Response:\n%+v", r)
		return err
	}
	glog.Infof("Deleted webhook %q successfully", webhookID)
	return nil
}

func (t *GithubEventSource) createWebhook(trigger sources.EventTrigger, domain string) (*sources.FeedContext, error) {

	glog.Infof("CREATING GITHUB WEBHOOK")

	accessToken, err := stringFrom(trigger.Parameters, accessTokenKey)
	if err != nil {
		return nil, fmt.Errorf("Failed to get access token from trigger parameters: %v", err)
	}

	secretToken, err := stringFrom(trigger.Parameters, secretTokenKey)
	if err != nil {
		return nil, fmt.Errorf("Failed to get secret token from trigger parameters: %v", err)
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := ghclient.NewClient(tc)
	active := true
	name := "web"
	config := make(map[string]interface{})
	config["url"] = fmt.Sprintf("http://%s", domain)
	config["content_type"] = "json"
	config["secret"] = secretToken
	// TODO(n3wscott): GitHub has many types of events, we will have to make this dynamic when we support
	// more types of events from GitHub.
	hook := ghclient.Hook{
		Name:   &name,
		URL:    &domain,
		Events: []string{"pull_request"},
		Active: &active,
		Config: config,
	}

	components := strings.Split(trigger.Resource, "/")
	owner := components[0]
	repo := components[1]
	h, r, err := client.Repositories.CreateHook(ctx, owner, repo, &hook)
	if err != nil {
		glog.Warningf("Failed to create the webhook: %s", err)
		glog.Warningf("Response:\n%+v", r)
		return nil, err
	}
	glog.Infof("Created hook: %+v", h)

	return &sources.FeedContext{
		Context: map[string]interface{}{
			webhookIDKey: strconv.FormatInt(*h.ID, 10),
		}}, nil
}

type parameters struct {
	Image string `json:"image,omitempty"`
}

// The main entry point for the GitHub Feedlet
func main() {
	flag.Parse()

	flag.Lookup("logtostderr").Value.Set("true")
	flag.Lookup("v").Value.Set("3")

	glog.Info("GitHub Feedlet starting...")

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
		glog.Fatalf("error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("error building kubernetes clientset: %s", err.Error())
	}

	servingClient, err := servingclientset.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("error building serving clientset: %s", err.Error())
	}

	sources.RunEventSource(NewGithubEventSource(kubeClient, servingClient, feedNamespace, feedServiceAccountName, p.Image))
	glog.Info("done...")
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
