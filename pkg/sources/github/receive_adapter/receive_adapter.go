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
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/golang/glog"
	webhooks "gopkg.in/go-playground/webhooks.v3"
	"gopkg.in/go-playground/webhooks.v3/github"

	"bytes"
	"io/ioutil"
	"net/http"

	ghclient "github.com/google/go-github/github"
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
	ctx    context.Context
	target string
}

type GithubSecrets struct {
	AccessToken string `json:"accessToken"`
	SecretToken string `json:"secretToken"`
}

// HandlePullRequest is invoked whenever a PullRequest is modified (created, updated, etc.)
func (h *GithubHandler) HandlePullRequest(payload interface{}, header webhooks.Header) {
	glog.Info("Handling Pull Request")

	pl := payload.(github.PullRequestPayload)

	glog.Infof("GOT PAYLOAD: %+v\n HEADER: %+v", pl, header)

	postMessage(h.target, &pl)
}

func main() {
	flag.Parse()
	// set the logs to stderr so kube will see them.
	flag.Lookup("logtostderr").Value.Set("true")

	target := os.Getenv(envTarget)

	glog.Infof("Target is: %q", target)

	githubSecrets := os.Getenv(envSecret)

	var credentials GithubSecrets
	err := json.Unmarshal([]byte(githubSecrets), &credentials)
	if err != nil {
		glog.Fatalf("Failed to unmarshal credentials: %s", err)
		return
	}

	// Set up the auth for being able to talk to Github. It's
	// odd that you have to also pass context around for the
	// calls even after giving it to client. But, whatever.
	ctx := context.Background()
	//ts := oauth2.StaticTokenSource(
	//	&oauth2.Token{AccessToken: credentials.AccessToken},
	//)
	//tc := oauth2.NewClient(ctx, ts)
	var tc *http.Client = nil

	client := ghclient.NewClient(tc)

	h := &GithubHandler{
		client: client,
		ctx:    ctx,
		target: target,
	}

	hook := github.New(&github.Config{Secret: credentials.SecretToken})
	hook.RegisterEvents(h.HandlePullRequest, github.PullRequestEvent)

	err = webhooks.Run(hook, ":8080", "/")
	if err != nil {
		glog.Fatalf("Failed to run the webhook")
	}
}

func postMessage(target string, m *github.PullRequestPayload) error {
	jsonStr, err := json.Marshal(m)
	if err != nil {
		glog.Errorf("Failed to marshal the message: %+v : %s", m, err)
		return err
	}

	URL := fmt.Sprintf("http://%s/", target)
	glog.Infof("Posting to %q", URL)
	req, err := http.NewRequest("POST", URL, bytes.NewBuffer(jsonStr))
	if err != nil {
		glog.Errorf("Failed to create http request: %s", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		glog.Errorf("Failed to do POST: %v", err)
		return err
	}
	defer resp.Body.Close()
	glog.Infof("response Status: %s", resp.Status)
	body, _ := ioutil.ReadAll(resp.Body)
	glog.Infof("response Body: %s", string(body))
	return nil
}
