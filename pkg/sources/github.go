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
	"errors"
	"fmt"
	"strings"

	"github.com/golang/glog"
	ghclient "github.com/google/go-github/github"
	"golang.org/x/oauth2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/elafros/eventing/pkg/apis/bind/v1alpha1"
	"github.com/elafros/eventing/pkg/delivery/action"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a Bind is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a Bind
	// is synced successfully
	MessageResourceSynced = "Bind synced successfully"
)

// GithubEventSource configures Event delivery from GitHub.
type GithubEventSource struct {
	clientset kubernetes.Interface
}

// NewGithubEventSource configures a new GithubEventSource.
func NewGithubEventSource(clientset kubernetes.Interface) EventSource {
	return &GithubEventSource{clientset: clientset}
}

// Bind implements EventSource.Bind
func (s *GithubEventSource) Bind(bind *v1alpha1.Bind, parameters map[string]interface{}) (map[string]interface{}, error) {
	glog.Infof("BINDING GITHUB EVENT")
	// BUG(#43) The GitHub event source should be responsible for its own gateway rathan relying
	// on elafros routes to be externally visible.
	if bind.Spec.Action.Processor != action.ElafrosActionType {
		return nil, errors.New("[elafros/eventing#43] GitHub event source currently only supports Elafros Routes as their action type")
	}

	address, err := s.getFunctionAddress(bind)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: parameters["accessToken"].(string)},
	)
	tc := oauth2.NewClient(ctx, ts)
	glog.Infof("CREATING GITHUB WEBHOOK with access token: %s", parameters["accessToken"].(string))

	client := ghclient.NewClient(tc)
	active := true
	name := "web"
	config := make(map[string]interface{})
	config["url"] = fmt.Sprintf("http://%s", address)
	config["content_type"] = "json"
	config["secret"] = parameters["secretToken"].(string)
	hook := ghclient.Hook{
		Name:   &name,
		URL:    &address,
		Events: []string{"pull_request"},
		Active: &active,
		Config: config,
	}

	components := strings.Split(bind.Spec.Trigger.Resource, "/")
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

func (s *GithubEventSource) getFunctionAddress(bind *v1alpha1.Bind) (string, error) {
	suffix, err := s.getDomainSuffix()
	if err != nil {
		return "", err
	}

	functionDNS := fmt.Sprintf(
		"%s.%s.%s",
		/* function name */ bind.Spec.Action.Name,
		/* function namespace */ bind.ObjectMeta.Namespace,
		/* cluster root domain */ suffix)
	glog.Infof("Found route DNS as '%q'", functionDNS)
	return functionDNS, nil
}

func (s *GithubEventSource) getDomainSuffix() (string, error) {
	const name = "ela-config"
	const namespace = "ela-system"
	c, err := s.clientset.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	domainSuffix, ok := c.Data["domainSuffix"]
	if !ok {
		return "", fmt.Errorf("cannot find domainSuffix in %v: ConfigMap.Data is %#v", name, c.Data)
	}
	return domainSuffix, nil
}
