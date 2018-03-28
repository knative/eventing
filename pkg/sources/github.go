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

package sources

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang/glog"

	"golang.org/x/oauth2"

	ghclient "github.com/google/go-github/github"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a Bind is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a Bind
	// is synced successfully
	MessageResourceSynced = "Bind synced successfully"
)

type GithubEventSource struct {
}

func NewGithubEventSource() EventSource {
	return &GithubEventSource{}
}

func (t *GithubEventSource) Bind(trigger EventTrigger, route string) (map[string]interface{}, error) {
	glog.Infof("CREATING GITHUB WEBHOOK")

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: trigger.Parameters["accessToken"].(string)},
	)
	tc := oauth2.NewClient(ctx, ts)
	glog.Infof("CREATING GITHUB WEBHOOK with access token: %s", trigger.Parameters["accessToken"].(string))

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
	h, r, err := client.Repositories.CreateHook(ctx, components[0] /* owner */, components[1] /* repo */, &hook)
	if err != nil {
		glog.Warningf("Failed to create the webhook: %s", err)
		glog.Warningf("Response:\n%+v", r)
		return nil, err
	}
	glog.Infof("Created hook: %+v", h)
	ret := make(map[string]interface{})
	ret["ID"] = fmt.Sprintf("%d", h.ID)
	return ret, nil
}
