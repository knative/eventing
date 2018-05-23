package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	// Imports the Google Cloud Pub/Sub client package.
	"cloud.google.com/go/pubsub"
	"golang.org/x/net/context"
)

const (
	// Environment variable containing project id
	envProject = "PROJECT_ID"
	// Target for messages
	envTarget = "TARGET"
	// Name of the subscription to use
	envSubscription = "SUBSCRIPTION"
)

func main() {
	flag.Parse()

	projectID := os.Getenv(envProject)
	target := os.Getenv(envTarget)
	subscriptionName := os.Getenv(envSubscription)

	log.Printf("projectid is: %q", projectID)
	log.Printf("subscriptionName is: %q", subscriptionName)
	log.Printf("Target is: %q", target)

	ctx := context.Background()

	// Creates a client.
	// TODO: Support additional ways of specifying the credentials for creating.
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	sub := client.Subscription(subscriptionName)

	err = sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
		log.Printf("Got message: %s", m.Data)
		err = postMessage(target, m)
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

func postMessage(target string, m *pubsub.Message) error {
	jsonStr, err := json.Marshal(m)
	if err != nil {
		log.Printf("Failed to marshal the message: %+v : %s", m, err)
		return err
	}

	URL := fmt.Sprintf("http://%s/", target)
	log.Printf("Posting to %q", URL)
	req, err := http.NewRequest("POST", URL, bytes.NewBuffer(jsonStr))
	if err != nil {
		log.Printf("Failed to create http request: %s", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")
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
