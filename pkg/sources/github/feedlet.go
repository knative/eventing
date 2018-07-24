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
	"log"
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
	accessToken = "accessToken"
	secretToken = "secretToken"

	nameSecretKeyRef = "secretName"
	keySecretKeyRef  = "secretKey"

	postfixReceiveAdapter = "rcvadptr"
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

	secretKeyRef map[string]string
}

func NewGithubEventSource(kubeclientset kubernetes.Interface, servingclientset servingclientset.Interface, feedNamespace, feedServiceAccountName, image string) sources.EventSource {
	return &GithubEventSource{
		kubeclientset:          kubeclientset,
		servingclientset:       servingclientset,
		feedNamespace:          feedNamespace,
		feedServiceAccountName: feedServiceAccountName,
		image: image,
		secretKeyRef: map[string]string{
			nameSecretKeyRef: "githubsecret-n3wscott",
			keySecretKeyRef:  "githubCredentials",
		},
	}
}

func (t *GithubEventSource) StopFeed(trigger sources.EventTrigger, feedContext sources.FeedContext) error {
	log.Printf("Stopping github webhook feed with context %+v", feedContext)

	serviceName := receiveAdapterName(trigger.Resource)
	t.deleteReceiveAdapter(t.feedNamespace, serviceName)

	components := strings.Split(trigger.Resource, "/")
	owner := components[0]
	repo := components[1]

	if _, ok := feedContext.Context[webhookIDKey]; !ok {
		// there's no webhook id, nothing to do.
		log.Printf("No Webhook ID Found, bailing...")
		return nil
	}
	webhookID := feedContext.Context[webhookIDKey].(string)

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: trigger.Parameters[accessToken].(string)},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := ghclient.NewClient(tc)

	id, err := strconv.ParseInt(webhookID, 10, 64)
	if err != nil {
		log.Printf("Failed to convert webhook %q to int64 : %s", webhookID, err)
		return err
	}
	_, err = client.Repositories.DeleteHook(ctx, owner, repo, id)
	if err != nil {
		if errResp, ok := err.(*ghclient.ErrorResponse); ok {
			// If the webhook doesn't exist, nothing to do
			if errResp.Message == "Not Found" {
				log.Printf("Webhook doesn't exist, nothing to delete.")
				return nil
			}
		}
		log.Printf("Failed to delete the webhook: %#v", err)
		return err
	}
	log.Printf("Deleted webhook %q successfully", webhookID)
	return nil
}

func (t *GithubEventSource) StartFeed(trigger sources.EventTrigger, target string) (*sources.FeedContext, error) {

	// Create the Receive Adapter Service that will accept incoming requests from GitHub.
	service, err := t.createReceiveAdapter(trigger, target)
	if err != nil {
		glog.Warningf("Failed to create service: %v", err)
		return nil, err
	}

	glog.Infof("Created Service: %+v", service)

	// Start watching the Receive Adapter Service for it's updated domain name. This will be passed
	// to GitHub as part of the webhook registration.
	receiveAdaptorDomain, err := t.waitForServiceDomain(service.GetObjectMeta().GetName())
	if err != nil {
		glog.Infof("Failed to get the service: %v", err)
	}

	return t.createWebhook(trigger, receiveAdaptorDomain)
}

func (t *GithubEventSource) waitForServiceDomain(serviceName string) (string, error) {

	sc := t.servingclientset.ServingV1alpha1().Services(t.feedNamespace)

	var watchTimeout int64 = 60 * 5 // 5 minutes?

	opts := metav1.ListOptions{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1alpha1",
		},
		FieldSelector:  fmt.Sprintf("metadata.name=%s", serviceName),
		LabelSelector:  "receive-adapter=github",
		Watch:          true,
		TimeoutSeconds: &watchTimeout,
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
	return serviceName
}

func (t *GithubEventSource) createReceiveAdapter(trigger sources.EventTrigger, target string) (*v1alpha1.Service, error) { //serviceName, image, route, serviceAccountName, githubSecret, githubSecretKey string) error {
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

	service := resources.MakeService(t.feedNamespace, serviceName, t.image, t.feedServiceAccountName, target, trigger.Parameters[nameSecretKeyRef].(string), trigger.Parameters[keySecretKeyRef].(string))
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

type parameters struct {
	Image string `json:"image,omitempty"`
}

func main() {
	flag.Parse()

	flag.Lookup("logtostderr").Value.Set("true")
	flag.Lookup("v").Value.Set("3")

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
	log.Printf("done...")
}
