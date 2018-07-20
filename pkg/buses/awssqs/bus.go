/*
 * Copyright 2018 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package awssqs

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"

	"github.com/golang/glog"
	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	"github.com/knative/eventing/pkg/buses"
)

type AWSSQSBus struct {
	name           string
	monitor        *buses.Monitor
	sqsClient      *sqs.SQS
	client         *http.Client
	forwardHeaders []string
	receivers      map[string]context.CancelFunc
}

const (
	queueUrlName   = "queueUrl"
	waitTimeSecond = 5
)

func (b *AWSSQSBus) CreateQueue(channel *channelsv1alpha1.Channel, parameters buses.ResolvedParameters) error {
	queueUrl, err := b.queueUrl(channel)
	// don't recreate queue
	if err == nil {
		glog.Infof("queue url %s exists, not recreating", queueUrl)
		return nil
	}
	queueID := b.queueID(channel)
	param := &sqs.CreateQueueInput{
		QueueName: &queueID,
	}
	glog.Infof("create queue %s", queueID)
	_, err = b.sqsClient.CreateQueue(param)
	if err != nil {
		glog.Warningf("failed to create queue %v", err)
		return err
	}
	return nil
}

func (b *AWSSQSBus) DeleteQueue(channel *channelsv1alpha1.Channel) error {
	glog.Infof("delete queue %s", b.queueID(channel))
	queueUrl, err := b.queueUrl(channel)
	if err != nil {
		return err
	}
	param := &sqs.DeleteQueueInput{
		QueueUrl: &queueUrl,
	}

	_, err = b.sqsClient.DeleteQueue(param)
	if err != nil {
		glog.Warningf("failed to delete queue %s: %v", queueUrl, err)
		return err
	}

	return nil
}

func (b *AWSSQSBus) SendMessageToQueue(channel *channelsv1alpha1.Channel, data []byte, attributes map[string]string) error {
	queueID := b.queueID(channel)
	queueUrl, err := b.queueUrl(channel)
	if err != nil {
		return err
	}

	// SQS only sends string
	msg := string(data[:])
	param := &sqs.SendMessageInput{
		MessageBody: aws.String(msg),
		QueueUrl:    aws.String(queueUrl),
	}
	resp, err := b.sqsClient.SendMessage(param)
	if err != nil {
		glog.Warningf("failed to send: %v", err)
		return err
	}

	glog.Infof("sent a message to %s; msg ID: %v\n", queueID, *resp.MessageId)
	return nil
}

func (b *AWSSQSBus) ReceiveMessages(sub *channelsv1alpha1.Subscription, parameters buses.ResolvedParameters) error {
	channel := b.monitor.Channel(sub.Spec.Channel, sub.Namespace)
	if channel == nil {
		return fmt.Errorf("Cannot create a Subscription for unknown Channel %q", sub.Spec.Channel)
	}
	queueUrl, err := b.queueUrl(channel)
	if err != nil {
		return fmt.Errorf("Cannot find queue url for Channel %q: %v", sub.Spec.Channel, err)
	}
	// cancel current subscription receiver, if any
	b.StopReceiveMessages(sub)

	ctx := context.Background()
	cctx, cancel := context.WithCancel(ctx)

	subscriptionID := b.subscriptionID(sub)
	b.receivers[subscriptionID] = cancel

	go func() {
		glog.Infof("Start receiving messages")
		for {
			rcvParam := &sqs.ReceiveMessageInput{
				QueueUrl:        aws.String(queueUrl),
				WaitTimeSeconds: aws.Int64(waitTimeSecond),
			}
			resp, err := b.sqsClient.ReceiveMessageWithContext(cctx, rcvParam)
			if err != nil {
				glog.Warningf("failed to receive from queue %s:%v", queueUrl, err)
				break
			}

			glog.V(5).Infof("received msg: %v", resp)

			for _, message := range resp.Messages {
				err := b.DispatchHTTPEvent(sub, []byte(*message.Body), message.Attributes)
				// delete message
				deleteParam := &sqs.DeleteMessageInput{
					QueueUrl:      aws.String(queueUrl),
					ReceiptHandle: message.ReceiptHandle,
				}
				_, err = b.sqsClient.DeleteMessage(deleteParam)
				if err != nil {
					glog.Warningf("failed to delete msg %v: %v", *message.MessageId, err)
				}
				glog.Infof("Message %v has beed deleted", *message.MessageId)
			}
		}
		delete(b.receivers, subscriptionID)
		b.monitor.RequeueSubscription(sub)

	}()

	return nil
}

func (b *AWSSQSBus) StopReceiveMessages(subscription *channelsv1alpha1.Subscription) error {
	subscriptionID := b.subscriptionID(subscription)
	if cancel, ok := b.receivers[subscriptionID]; ok {
		glog.Infof("Stop receiving events for subscription %q\n", subscriptionID)
		cancel()
		delete(b.receivers, subscriptionID)
	}
	return nil
}

func (b *AWSSQSBus) queueUrl(channel *channelsv1alpha1.Channel) (string, error) {
	// get queue url via sqs call
	queueName := b.queueID(channel)
	param := &sqs.ListQueuesInput{
		QueueNamePrefix: &queueName,
	}
	resp, err := b.sqsClient.ListQueues(param)
	if err == nil {
		for i := range resp.QueueUrls {
			q := *resp.QueueUrls[i]
			// if both prefix and suffix match, return it
			if strings.HasSuffix(q, queueName) {
				return q, nil
			}
		}
		return "", fmt.Errorf("no such queue: %s", queueName)
	} else {
		return "", fmt.Errorf("failed to find queue url %s: %v", queueName, err)
	}
}

func (b *AWSSQSBus) queueID(channel *channelsv1alpha1.Channel) string {
	return fmt.Sprintf("channel-%s-%s-%s", b.name, channel.Namespace, channel.Name)
}

func (b *AWSSQSBus) subscriptionID(subscription *channelsv1alpha1.Subscription) string {
	return fmt.Sprintf("subscription-%s-%s-%s", b.name, subscription.Namespace, subscription.Name)
}

func (b *AWSSQSBus) ReceiveHTTPEvent(res http.ResponseWriter, req *http.Request) {
	host := req.Host
	glog.Infof("Received request for %s\n", host)

	name, namespace := b.splitChannelName(host)
	channel := b.monitor.Channel(name, namespace)
	if channel == nil {
		res.WriteHeader(http.StatusNotFound)
		return
	}

	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	attributes := b.headersToAttributes(b.safeHeaders(req.Header))

	err = b.SendMessageToQueue(channel, data, attributes)
	if err != nil {
		glog.Warningf("Unable to send event to queue %q: %v", channel.Name, err)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.WriteHeader(http.StatusAccepted)
}

func (b *AWSSQSBus) DispatchHTTPEvent(subscription *channelsv1alpha1.Subscription, data []byte, attributes map[string]*string) error {
	subscriber := subscription.Spec.Subscriber
	url := url.URL{
		Scheme: "http",
		Host:   subscription.Spec.Subscriber,
		Path:   "/",
	}
	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header = b.safeHeaders(b.attributesToHeaders(attributes))
	res, err := b.client.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("Subscribing service %q did not accept event: got HTTP %d", subscriber, res.StatusCode)
	}
	return nil
}

func (b *AWSSQSBus) splitChannelName(host string) (string, string) {
	chunks := strings.Split(host, ".")
	channel := chunks[0]
	namespace := chunks[1]
	return channel, namespace
}

func (b *AWSSQSBus) safeHeaders(raw http.Header) http.Header {
	safe := http.Header{}
	for _, header := range b.forwardHeaders {
		if value := raw.Get(header); value != "" {
			safe.Set(header, value)
		}
	}
	return safe
}

func (b *AWSSQSBus) headersToAttributes(headers http.Header) map[string]string {
	attributes := make(map[string]string)
	for name, value := range headers {
		// TODO hanle compound headers
		attributes[name] = value[0]
	}
	return attributes
}

func (b *AWSSQSBus) attributesToHeaders(attributes map[string]*string) http.Header {
	headers := http.Header{}
	for name, value := range attributes {
		headers.Set(name, *value)
	}
	return headers
}

func NewAWSSQSBus(name, region, credFile, credProfile string, monitor *buses.Monitor) (*AWSSQSBus, error) {
	forwardHeaders := []string{
		"content-type",
		"x-request-id",
		"x-b3-traceid",
		"x-b3-spanid",
		"x-b3-parentspanid",
		"x-b3-sampled",
		"x-b3-flags",
		"x-ot-span-context",
	}

	sess := session.New(&aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewSharedCredentials(credFile, credProfile),
	})
	sqsClient := sqs.New(sess)
	glog.Infof("sqs client created %+v", sqsClient)
	bus := AWSSQSBus{
		name:           name,
		sqsClient:      sqsClient,
		client:         &http.Client{},
		monitor:        monitor,
		forwardHeaders: forwardHeaders,
		receivers:      map[string]context.CancelFunc{},
	}

	return &bus, nil
}
