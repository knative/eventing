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
	"strings"

	"github.com/google/uuid"
	"github.com/knative/eventing/pkg/sources"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
)

const (
	deployment = "deployment"
)

type AWSSQSEventSource struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	image         string
	// namespace where the feeding is created
	feedNamespace string
	// serviceAccount that the container runs as. Launches Receive Adapter with the
	// same Service Account
	feedServiceAccountName string
}

func NewAWSSQSEventSource(kubeclientset kubernetes.Interface, feedNamespace string, feedServiceAccountName string, image string) sources.EventSource {
	log.Printf("Image: %q", image)
	return &AWSSQSEventSource{kubeclientset: kubeclientset, feedNamespace: feedNamespace, feedServiceAccountName: feedServiceAccountName, image: image}
}

func (a *AWSSQSEventSource) StopFeed(trigger sources.EventTrigger, feedContext sources.FeedContext) error {
	log.Printf("Stopping AWS SQS Events feed with context %+v", feedContext)

	if deploymentName, ok := feedContext.Context[deployment].(string); ok {
		err := a.deleteAWSSQSDeployment(deploymentName)
		if err != nil {
			log.Printf("Failed to delete deployment: %s", err)
			return err
		}
	}
	return nil
}

func (a *AWSSQSEventSource) StartFeed(trigger sources.EventTrigger, route string) (*sources.FeedContext, error) {
	log.Printf("creating awssqs feed context")
	// create aws sqs deployment
	awsToken := ""
	awsId := trigger.Parameters["AWS_ACCESS_KEY_ID"].(string)
	awsKey := trigger.Parameters["AWS_SECRET_ACCESS_KEY"].(string)
	if token, ok := trigger.Parameters["AWS_SESSION_TOKEN"].(string); ok {
		awsToken = token
	}
	if len(awsId) == 0 && len(awsKey) == 0 {
		return nil, fmt.Errorf("missing AWS_ACCESS_KEY_ID or AWS_SECRET_ACCESS_KEY")
	}
	resource := strings.Split(trigger.Resource, "/")
	if len(resource) != 2 {
		return nil, fmt.Errorf("invalid resource: must be region/queue-name")
	}
	region := resource[0]
	queueName := resource[1]

	// create aws sqs client
	sess := session.New(&aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewStaticCredentials(awsId, awsKey, awsToken),
	})
	sqsClient := sqs.New(sess)
	// list queue
	param := &sqs.ListQueuesInput{
		QueueNamePrefix: &queueName,
	}
	resp, err := sqsClient.ListQueues(param)
	if err != nil {
		log.Printf("failed to list queue %q: %v", queueName, err)
		return nil, fmt.Errorf("failed to list queue %q: %v", queueName, err)
	}
	queueUrl := ""
	for i := range resp.QueueUrls {
		q := *resp.QueueUrls[i]
		// if both prefix and suffix match, return it
		if strings.HasSuffix(q, queueName) {
			queueUrl = q
			break
		}
	}
	// if queue not found, create one
	if len(queueUrl) == 0 {
		param := &sqs.CreateQueueInput{
			QueueName: &queueName,
		}
		log.Printf("create queue %s", queueName)
		resp, err := sqsClient.CreateQueue(param)
		if err != nil {
			return nil, fmt.Errorf("failed to create queue %v", err)
		}
		queueUrl = *resp.QueueUrl
	}

	// create a random deployment name
	uuid, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	deploymentName := "awssqs-" + queueName + "-" + uuid.String()
	err = a.createAWSSQSDeployment(deploymentName, a.image, awsId, awsKey, awsToken, region, queueUrl, route)
	if err != nil {
		log.Printf("Failed to create deployment: %s", err)
		return nil, err
	}
	return &sources.FeedContext{
		Context: map[string]interface{}{
			deployment: deploymentName,
		}}, nil
}

func (a *AWSSQSEventSource) createAWSSQSDeployment(deploymentName, image, awsId, awsKey, awsToken, region, queueUrl, route string) error {
	dc := a.kubeclientset.AppsV1().Deployments(a.feedNamespace)

	// First, check if deployment exists already.
	if _, err := dc.Get(deploymentName, metav1.GetOptions{}); err != nil {
		if !apierrs.IsNotFound(err) {
			log.Printf("deployments.Get for %q failed: %v", deploymentName, err)
			return err
		}
		log.Printf("Deployment %q doesn't exist, creating", deploymentName)
	} else {
		log.Printf("Found existing deployment %q", deploymentName)
		return nil
	}

	deployment := MakeAWSSQSDeployment(a.feedNamespace, deploymentName, image, awsId, awsKey, awsToken, region, queueUrl, route)
	_, createErr := dc.Create(deployment)
	return createErr
}

func (a *AWSSQSEventSource) deleteAWSSQSDeployment(deploymentName string) error {
	dc := a.kubeclientset.AppsV1().Deployments(a.feedNamespace)

	// First, check if deployment exists already.
	if _, err := dc.Get(deploymentName, metav1.GetOptions{}); err != nil {
		if !apierrs.IsNotFound(err) {
			log.Printf("deployments.Get for %q failed: %v", deploymentName, err)
			return err
		}
		log.Printf("Deployment %q already deleted", deploymentName)
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
		log.Printf("can not unmarshal %q : %v", decodedParameters, err)
		return
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		log.Printf("Error building kubeconfig: %v", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Printf("Error building kubernetes clientset: %v", err.Error())
	}

	err = sources.RunEventSource(NewAWSSQSEventSource(kubeClient, feedNamespace, feedServiceAccountName, p.Image))
	if err != nil {
		log.Printf("failed to create event: %v", err)
	}
	log.Printf("Done...")
}
