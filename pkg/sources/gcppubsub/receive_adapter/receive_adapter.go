package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	// Imports the Google Cloud Pub/Sub client package.
	"cloud.google.com/go/pubsub"
	"github.com/knative/pkg/cloudevents"
	"golang.org/x/net/context"
)

const (
	// Environment variable containing project id
	envProject = "PROJECT_ID"

	// Environment variable containing topic id
	envTopic = "TOPIC_ID"

	// Target for messages
	envTarget = "TARGET"

	// Name of the subscription to use
	envSubscription = "SUBSCRIPTION"
)

func main() {
	flag.Parse()

	projectID := os.Getenv(envProject)
	topicID := os.Getenv(envTopic)
	target := os.Getenv(envTarget)
	subscriptionName := os.Getenv(envSubscription)

	log.Printf("projectID is: %q", projectID)
	log.Printf("topicID is: %q", topicID)
	log.Printf("subscriptionName is: %q", subscriptionName)
	log.Printf("Target is: %q", target)

	ctx := context.Background()
	source := fmt.Sprintf("//pubsub.googleapis.com/projects/%s/topics/%s", projectID, topicID)

	// Creates a client.
	// TODO: Support additional ways of specifying the credentials for creating.
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	sub := client.Subscription(subscriptionName)

	err = sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
		log.Printf("Got message: %s", m.Data)
		err = postMessage(target, source, m)
		if err != nil {
			log.Printf("Failed to post message: %s", err)
			m.Nack()
		} else {
			m.Ack()
		}
	})
	if err != nil {
		log.Fatalf("Failed to create receive: %v", err)
	}
}

func postMessage(target string, source string, m *pubsub.Message) error {
	URL := fmt.Sprintf("http://%s/", target)
	ctx := cloudevents.EventContext{
		CloudEventsVersion: cloudevents.CloudEventsVersion,
		EventType:          "google.pubsub.topic.publish",
		EventID:            m.ID,
		EventTime:          m.PublishTime,
		Source:             source,
	}
	req, err := cloudevents.Binary.NewRequest(URL, m, ctx)
	if err != nil {
		log.Printf("Failed to marshal the message: %+v : %s", m, err)
		return err
	}

	log.Printf("Posting to %q", URL)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	log.Printf("response Status: %s", resp.Status)
	body, _ := ioutil.ReadAll(resp.Body)
	log.Printf("response Body: %s", string(body))
	return nil
}
