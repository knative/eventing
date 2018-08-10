/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"gopkg.in/go-playground/webhooks.v3"
	gh "gopkg.in/go-playground/webhooks.v3/github"

	"io/ioutil"
	"net/http"

	ghclient "github.com/google/go-github/github"
	"github.com/google/uuid"
	"github.com/knative/eventing/pkg/event"
	"github.com/knative/eventing/pkg/sources/github"
	"log"
	"time"
)

const (
	// Target for messages
	envTarget = "TARGET"
	// Environment variable containing json credentials
	envSecret = "GITHUB_SECRET"
)

// GithubHandler holds necessary objects for communicating with the Github.
type GithubHandler struct {
	client *ghclient.Client
	target string
}

type GithubSecrets struct {
	AccessToken string `json:"accessToken"`
	SecretToken string `json:"secretToken"`
}

// HandlePullRequest is invoked whenever a PullRequest is modified (created, updated, etc.)
func (h *GithubHandler) HandlePullRequest(payload interface{}, header webhooks.Header) {
	log.Print("Handling Pull Request")

	hdr := http.Header(header)

	pl := payload.(gh.PullRequestPayload)

	source := pl.PullRequest.HTMLURL

	eventType, ok := github.CloudEventType[hdr.Get("X-GitHub-Event")]
	if !ok {
		eventType = github.UnsupportedEvent
	}

	eventID := hdr.Get("X-GitHub-Delivery")
	if len(eventID) == 0 {
		if uuid, err := uuid.NewRandom(); err != nil {
			eventID = uuid.String()
		}
	}

	postMessage(h.target, payload, source, eventType, eventID)
}

func main() {
	flag.Parse()
	// set the logs to stderr so kube will see them.
	flag.Lookup("logtostderr").Value.Set("true")

	target := os.Getenv(envTarget)

	log.Printf("Target is: %q", target)

	githubSecrets := os.Getenv(envSecret)

	var credentials GithubSecrets
	err := json.Unmarshal([]byte(githubSecrets), &credentials)
	if err != nil {
		log.Fatalf("Failed to unmarshal credentials: %v", err)
		return
	}

	// Set up the auth for being able to talk to Github.
	var tc *http.Client = nil

	client := ghclient.NewClient(tc)

	h := &GithubHandler{
		client: client,
		target: target,
	}

	hook := gh.New(&gh.Config{Secret: credentials.SecretToken})
	// TODO: GitHub has more than just Pull Request Events. This needs to
	// handle them all?
	hook.RegisterEvents(h.HandlePullRequest, gh.PullRequestEvent)

	// TODO(n3wscott): Do we need to configure the PORT?
	err = webhooks.Run(hook, ":8080", "/")
	if err != nil {
		log.Fatalf("Failed to run the webhook")
	}
}

func postMessage(target string, payload interface{}, source, eventType, eventID string) error {
	URL := fmt.Sprintf("http://%s/", target)

	ctx := event.EventContext{
		CloudEventsVersion: event.CloudEventsVersion,
		EventType:          eventType,
		EventID:            eventID,
		EventTime:          time.Now(),
		Source:             source,
	}
	req, err := event.Binary.NewRequest(URL, payload, ctx)
	if err != nil {
		log.Printf("Failed to marshal the message: %+v : %s", payload, err)
		return err
	}

	log.Printf("Posting to %q", URL)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// TODO: in general, receive adapters may have to be able to retry for error cases.
		log.Printf("response Status: %s", resp.Status)
		body, _ := ioutil.ReadAll(resp.Body)
		log.Printf("response Body: %s", string(body))
	}
	return nil
}
