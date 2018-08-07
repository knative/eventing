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
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/knative/pkg/signals"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
)

const (
	// Target for messages
	envTarget = "TARGET"
	// aws id
	envAWSId = "AWS_ACCESS_KEY_ID"
	// aws key
	envAWSKey = "AWS_SECRET_ACCESS_KEY"
	// aws token
	envAWSToken = "AWS_SESSION_TOKEN"
	// aws region
	envAWSRegion = "AWS_REGION"
	// aws sqs queue url
	envAWSSQSQueue = "AWS_SQS_QUEUE_URL"
	// message receive timeout
	waitTimeSecond = 5
)

var (
	// forward headers
	forwardHeaders = []string{
		"content-type",
		"x-request-id",
		"x-b3-traceid",
		"x-b3-spanid",
		"x-b3-parentspanid",
		"x-b3-sampled",
		"x-b3-flags",
		"x-ot-span-context",
	}
)

type message struct {
	Message string
}

func main() {
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	target := os.Getenv(envTarget)
	awsId := os.Getenv(envAWSId)
	awsKey := os.Getenv(envAWSKey)
	awsToken := os.Getenv(envAWSToken)
	awsRegion := os.Getenv(envAWSRegion)
	queueUrl := os.Getenv(envAWSSQSQueue)

	log.Printf("Target is: %q", target)

	// create aws sqs client
	sess := session.New(&aws.Config{
		Region:      aws.String(awsRegion),
		Credentials: credentials.NewStaticCredentials(awsId, awsKey, awsToken),
	})
	sqsClient := sqs.New(sess)

	log.Printf("Start receiving messages")
	ctx := context.Background()
	cctx, cancel := context.WithCancel(ctx)
	go func() {
		for {
			rcvParam := &sqs.ReceiveMessageInput{
				QueueUrl:        aws.String(queueUrl),
				WaitTimeSeconds: aws.Int64(waitTimeSecond),
			}
			resp, err := sqsClient.ReceiveMessageWithContext(cctx, rcvParam)
			if err != nil {
				log.Printf("failed to receive from queue %s:%v", queueUrl, err)
				continue
			}

			log.Printf("received msg: %v", resp)

			for _, message := range resp.Messages {
				err := postMessage(target, []byte(*message.Body), message.Attributes)
				if err != nil {
					// retry
					log.Printf("failed to post message %s:%v", err)
					continue
				}
				// delete message
				deleteParam := &sqs.DeleteMessageInput{
					QueueUrl:      aws.String(queueUrl),
					ReceiptHandle: message.ReceiptHandle,
				}
				_, err = sqsClient.DeleteMessage(deleteParam)
				if err != nil {
					log.Printf("failed to delete msg %v: %v", *message.MessageId, err)
				}
				log.Printf("Message %v has beed deleted", *message.MessageId)
			}
		}
	}()
	<-stopCh
	cancel()
	log.Printf("exiting")
}

func postMessage(target string, data []byte, attributes map[string]*string) error {
	url := url.URL{
		Scheme: "http",
		Host:   target,
		Path:   "/",
	}
	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating http request failed: %v", err)
	}
	req.Header = safeHeaders(attributesToHeaders(attributes))
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending http request failed: %v", err)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("%q did not accept event: got HTTP %d", target, res.StatusCode)
	}
	return nil
}

func attributesToHeaders(attributes map[string]*string) http.Header {
	headers := http.Header{}
	for name, value := range attributes {
		headers.Set(name, *value)
	}
	return headers
}

func safeHeaders(raw http.Header) http.Header {
	safe := http.Header{}
	for _, header := range forwardHeaders {
		if value := raw.Get(header); value != "" {
			safe.Set(header, value)
		}
	}
	return safe
}
