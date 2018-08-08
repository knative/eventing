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
	"log"
	"time"
)

const (
	webhookIDKey = "id"

	// property bag keys
	accessTokenKey = "accessToken"
	secretTokenKey = "secretToken"
	eventName      = "event"

	// postfixReceiveAdapter is appended to the name of the service running the Receive Adapter
	postfixReceiveAdapter = "rcvadptr"

	// watchTimeout is the timeout that the feedlet will wait for the Receiver Adapter to get a domain name.
	watchTimeout = 5 * time.Minute

	// secretNameKey is the name of the secret that contains the GitHub credentials.
	secretNameKey = "secretName"
	// secretKeyKey is the name of key inside the secret that contains the GitHub credentials.
	secretKeyKey = "secretKey"
)

type githubEventSource struct {
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
	return &githubEventSource{
		kubeclientset:          kubeclientset,
		servingclientset:       servingclientset,
		feedNamespace:          feedNamespace,
		feedServiceAccountName: feedServiceAccountName,
		image: image,
	}
}

// TODO(n3wscott): Add a timeout for StopFeed.
func (t *githubEventSource) StopFeed(trigger sources.EventTrigger, feedContext sources.FeedContext) error {
	log.Printf("stopping github webhook feed with context %+v", feedContext)

	return t.deleteWebhook(trigger, feedContext)
}

// TODO(n3wscott): Add a timeout for StartFeed.
func (t *githubEventSource) StartFeed(trigger sources.EventTrigger, target string) (*sources.FeedContext, error) {

	// Create the Receive Adapter Service that will accept incoming requests from GitHub.
	service, err := t.createReceiveAdapter(trigger, target)
	if err != nil {
		return nil, fmt.Errorf("failed to create service: %v", err)
	}

	// TODO(n3wscott): look into using spew.
	log.Printf("created Service: %+v", service)

	// Start watching the Receive Adapter Service for it's updated domain name. This will be passed
	// to GitHub as part of the webhook registration.
	receiveAdapterDomain, err := t.waitForServiceDomain(service.GetObjectMeta().GetName())
	if err != nil {
		return nil, fmt.Errorf("failed to get the service: %v", err)
	}

	return t.createWebhook(trigger, service.GetObjectMeta().GetName(), receiveAdapterDomain)
}

func (t *githubEventSource) waitForServiceDomain(serviceName string) (string, error) {
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
		log.Printf("[%s] failed to create watch: %v", serviceName, err)
		return "", err
	}

	eventChan := w.ResultChan()
	for {
		event, more := <-eventChan
		if !more {
			return "", fmt.Errorf("[%s] watch channel closed, no more", serviceName)
		}
		switch event.Type {
		case watch.Error:
			return "", fmt.Errorf("[%s] watched Service error", serviceName)
		case watch.Deleted:
			return "", fmt.Errorf("[%s] watched Service deleted", serviceName)
		case watch.Added, watch.Modified:
			service, ok := event.Object.(*v1alpha1.Service)
			if !ok {
				log.Printf("[%s] expected a Service object, but got %T", serviceName, event.Object)
				continue
			}
			if service.Name != serviceName {
				log.Printf("Error: [%s] expected a service.Name %q to match expected serviceName %q",
					serviceName, service.Name, serviceName)
				continue
			}
			status := service.Status
			if status.Domain != "" {
				w.Stop()
				return status.Domain, nil
			}

		}
	}
}

func receiveAdapterName(trigger sources.EventTrigger) string {
	// TODO(n3wscott): this needs more UUID on the end of it?
	// TODO(n3wscott): Currently this needs to be deterministic so StopFeed can find the receive adapter. If the receive
	// adapter name were added to the feed context, then this could be a uuid.
	serviceName := fmt.Sprintf("%s-%s-%s", "github", trigger.Resource, postfixReceiveAdapter)
	serviceName = strings.Replace(serviceName, "/", "-", -1)
	serviceName = strings.Replace(serviceName, ".", "-", -1)
	serviceName = strings.ToLower(serviceName)
	return serviceName
}

func (t *githubEventSource) createReceiveAdapter(trigger sources.EventTrigger, target string) (*v1alpha1.Service, error) {
	sc := t.servingclientset.ServingV1alpha1().Services(t.feedNamespace)

	serviceName := receiveAdapterName(trigger)

	// First, check if service exists already.
	if sc, err := sc.Get(serviceName, metav1.GetOptions{}); err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, fmt.Errorf("service.Get for %q failed: %v", serviceName, err)
		}
		log.Printf("service %q doesn't exist, creating", serviceName)
	} else {
		log.Printf("found existing service %q", serviceName)
		// Don't try again.
		return sc, nil
	}

	secretName, err := stringFrom(trigger.Parameters, secretNameKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret name from trigger parameters: %v", err)
	}

	secretKey, err := stringFrom(trigger.Parameters, secretKeyKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret key from trigger parameters: %v", err)
	}

	service := resources.MakeService(t.feedNamespace, serviceName, t.image, t.feedServiceAccountName, target, secretName, secretKey)
	service, err = sc.Create(service)
	if err != nil {
		return nil, fmt.Errorf("failed to create service: %s", err)
	}

	return service, nil
}

func (t *githubEventSource) deleteReceiveAdapter(namespace string, serviceName string) error {
	sc := t.servingclientset.ServingV1alpha1().Services(t.feedNamespace)

	// First, check if deployment exists already.
	if _, err := sc.Get(serviceName, metav1.GetOptions{}); err != nil {
		if !apierrs.IsNotFound(err) {
			return fmt.Errorf("services.Get for %q failed: %v", serviceName, err)
		}
		log.Printf("service %q already deleted", serviceName)
		// Don't try again.
		return nil
	}

	return sc.Delete(serviceName, &metav1.DeleteOptions{})
}

func (t *githubEventSource) deleteWebhook(trigger sources.EventTrigger, feedContext sources.FeedContext) error {
	serviceName := receiveAdapterName(trigger)
	err := t.deleteReceiveAdapter(t.feedNamespace, serviceName)
	if err != nil {
		log.Printf("Error: Failed to delete the ReceiveAdapter \"%s/%s\": %v", t.feedNamespace, serviceName, err)
		// Continue deleting the webhook.
	}

	owner, repo, err := parseOwnerRepoFrom(trigger.Resource)
	if err != nil {
		log.Printf("Error: Failed to parse owner and repo from tigger.resource: %v; bailing...", err)
		// Don't try again.
		return nil
	}

	webhookID, err := webhookIDFrom(feedContext)
	if err != nil {
		log.Printf("Error: Failed to get webhook id from context: %v; bailing...", err)
		// Don't try again.
		return nil
	}

	accessToken, err := stringFrom(trigger.Parameters, accessTokenKey)
	if err != nil {
		log.Printf("Error: Failed to get access token from trigger parameters: %v; bailing...", err)
		// Don't try again.
		return nil
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := ghclient.NewClient(tc)

	_, err = client.Repositories.DeleteHook(ctx, owner, repo, webhookID)
	if err != nil {
		if errResp, ok := err.(*ghclient.ErrorResponse); ok && errResp.Message == "Not Found" {
			// If the webhook doesn't exist, nothing to do
			return fmt.Errorf("webhook doesn't exist, nothing to delete")
		}
		return fmt.Errorf("failed to delete the webhook: %v", err)
	}
	log.Printf("deleted webhook \"%d\" successfully", webhookID)
	return nil
}

func (t *githubEventSource) createWebhook(trigger sources.EventTrigger, name, domain string) (*sources.FeedContext, error) {

	log.Printf("CREATING GITHUB WEBHOOK")

	owner, repo, err := parseOwnerRepoFrom(trigger.Resource)
	if err != nil {
		return nil, err
	}

	events, err := parseEventsFrom(trigger.EventType)
	if err != nil {
		return nil, err
	}

	accessToken, err := stringFrom(trigger.Parameters, accessTokenKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token from trigger parameters: %v", err)
	}

	secretToken, err := stringFrom(trigger.Parameters, secretTokenKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret token from trigger parameters: %v", err)
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := ghclient.NewClient(tc)
	active := true
	config := make(map[string]interface{})
	config["url"] = fmt.Sprintf("http://%s", domain)
	config["content_type"] = "json"
	config["secret"] = secretToken
	hook := ghclient.Hook{
		Name:   &name,
		URL:    &domain,
		Events: events,
		Active: &active,
		Config: config,
	}

	h, r, err := client.Repositories.CreateHook(ctx, owner, repo, &hook)
	if err != nil {
		log.Printf("create webhook error response:\n%+v", r)
		return nil, fmt.Errorf("failed to create the webhook: %v", err)
	}
	log.Printf("created hook: %+v", h)

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

	log.Printf("GitHub Feedlet starting...")

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
		log.Printf("Error: error building kubeconfig: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Printf("Error: error building kubernetes clientset: %v", err)
	}

	servingClient, err := servingclientset.NewForConfig(cfg)
	if err != nil {
		log.Printf("Error: error building serving clientset: %v", err)
	}

	sources.RunEventSource(NewGithubEventSource(kubeClient, servingClient, feedNamespace, feedServiceAccountName, p.Image))
	log.Printf("done...")
}

func webhookIDFrom(feedContext sources.FeedContext) (int64, error) {
	webhookID, err := stringFrom(feedContext.Context, webhookIDKey)
	if err != nil {
		return 0, err

	}
	id, err := int64From(webhookID)
	if err != nil {
		return 0, fmt.Errorf("failed to convert webhook %q to int64 : %v", webhookID, err)
	}
	return id, nil
}

func int64From(strNum string) (int64, error) {
	return strconv.ParseInt(strNum, 10, 64)
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

func parseOwnerRepoFrom(resource string) (string, string, error) {
	if len(resource) == 0 {
		return "", "", fmt.Errorf("resouce is empty")
	}
	components := strings.Split(resource, "/")
	if len(components) != 2 {
		return "", "", fmt.Errorf("resouce is malformatted, expected 'owner/repo' but found %q", resource)
	}
	owner := components[0]
	if len(owner) == 0 {
		return "", "", fmt.Errorf("owner is empty, expected 'owner/repo' but found %q", resource)
	}
	repo := components[1]
	if len(repo) == 0 {
		return "", "", fmt.Errorf("repo is empty, expected 'owner/repo' but found %q", resource)
	}

	return owner, repo, nil
}

func parseEventsFrom(eventType string) ([]string, error) {
	if len(eventType) == 0 {
		return []string(nil), fmt.Errorf("event type is empty")
	}
	switch eventType {
	case "pullrequest":
		return []string{"pull_request"}, nil
	// TODO: Add more supported event types.
	default:
		return []string(nil), fmt.Errorf("event type is unknown: %s", eventType)
	}
}
