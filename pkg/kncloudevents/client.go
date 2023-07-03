/*
Copyright 2023 The Knative Authors

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

package kncloudevents

import (
	"context"
	nethttp "net/http"

	duckv1 "knative.dev/pkg/apis/duck/v1"
)

type Client interface {
	NewRequest(ctx context.Context, target duckv1.Addressable) (Request, error)
}

var _ Client = (*clientImpl)(nil)

func NewClient() Client {
	c := newClientImpl()
	return &c
}

type clientImpl struct{}

func newClientImpl() clientImpl {
	return clientImpl{}
}

func (c *clientImpl) NewRequest(ctx context.Context, target duckv1.Addressable) (Request, error) {
	nethttpReqest, err := nethttp.NewRequestWithContext(ctx, "POST", target.URL.String(), nil)
	if err != nil {
		return nil, err
	}

	return &requestImpl{
		Target:  target,
		Request: nethttpReqest,
	}, nil
}
