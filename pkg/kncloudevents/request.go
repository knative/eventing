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

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

type Request struct {
	*nethttp.Request
	target duckv1.Addressable
}

func NewRequest(ctx context.Context, target duckv1.Addressable) (*Request, error) {
	nethttpReqest, err := nethttp.NewRequestWithContext(ctx, "POST", target.URL.String(), nil)
	if err != nil {
		return nil, err
	}

	return &Request{
		Request: nethttpReqest,
		target:  target,
	}, nil
}

func (req *Request) HTTPRequest() *nethttp.Request {
	return req.Request
}

func (req *Request) BindEvent(ctx context.Context, event event.Event) error {
	message := binding.ToMessage(&event)
	defer message.Finish(nil)

	return http.WriteRequest(ctx, message, req.HTTPRequest())
}
