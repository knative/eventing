/*
Copyright 2018 Google, Inc. All rights reserved.

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

	"github.com/elafros/eventing/pkg/sources"
	"golang.org/x/oauth2"

	ghclient "github.com/google/go-github/github"
)

const (
	webhookIDKey = "id"
	ownerKey     = "owner"
	repoKey      = "repo"

	// SuccessSynced is used as part of the Event 'reason' when a Bind is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a Bind
	// is synced successfully
	MessageResourceSynced = "Bind synced successfully"
)

type GithubEventSource struct {
}

func NewGithubEventSource() sources.EventSource {
	return &GithubEventSource{}
}

func (t *GithubEventSource) Unbind(trigger sources.EventTrigger, bindContext sources.BindContext) error {
	glog.Infof("Unbinding github webhook with context %+v", bindContext)

	components := strings.Split(trigger.Resource, "/")
	owner := components[0]
	repo := components[1]

	if _, ok := bindContext.Context[webhookIDKey]; !ok {
		// there's no webhook id, nothing to do.
		glog.Infof("No Webhook ID Found, bailing...")
		return nil
	}
	webhookID := bindContext.Context[webhookIDKey].(string)

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: trigger.Parameters["accessToken"].(string)},
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
		glog.Warningf("Failed to delete the webhook: %s", err)
		glog.Warningf("Response:\n%+v", r)
		return err
	}
	glog.Infof("Deleted webhook %q successfully", webhookID)
	return nil
}

func (t *GithubEventSource) Bind(trigger sources.EventTrigger, route string) (*sources.BindContext, error) {
	glog.Infof("CREATING GITHUB WEBHOOK")

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: trigger.Parameters["accessToken"].(string)},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := ghclient.NewClient(tc)
	active := true
	name := "web"
	config := make(map[string]interface{})
	config["url"] = fmt.Sprintf("http://%s", route)
	config["content_type"] = "json"
	config["secret"] = trigger.Parameters["secretToken"].(string)
	hook := ghclient.Hook{
		Name:   &name,
		URL:    &route,
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
	return &sources.BindContext{
		Context: map[string]interface{}{
			webhookIDKey: strconv.FormatInt(*h.ID, 10),
		}}, nil
}

func main() {
	flag.Parse()

	sources.RunEventSource(NewGithubEventSource())
	log.Printf("Done...")
}
