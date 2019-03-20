/*
 * Copyright 2019 The Knative Authors
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

package broker

import (
	"context"
	"net/url"
	"strings"

	cecontext "github.com/cloudevents/sdk-go/pkg/cloudevents/context"
	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"k8s.io/apimachinery/pkg/util/sets"
)

var (
	// These MUST be lowercase strings, as they will be compared against lowercase strings.
	forwardHeaders = sets.NewString(
		// tracing
		"x-request-id",
	)
	// These MUST be lowercase strings, as they will be compared against lowercase strings.
	forwardPrefixes = []string{
		// knative
		"knative-",
		// tracing
		"x-b3-",
		"x-ot-",
	}
)

// SendingContext creates the context to use when sending a Cloud Event with ceclient.Client. It
// sets the target and attaches a filtered set of headers from the initial request.
func SendingContext(ctx context.Context, tctx cehttp.TransportContext, targetURI *url.URL) context.Context {
	sendingCTX := cecontext.WithTarget(ctx, targetURI.String())

	// Helper function that adds the header name and all its values.
	addHeader := func(c context.Context, n string, v []string) context.Context {
		for _, iv := range v {
			c = cehttp.ContextWithHeader(c, n, iv)
		}
		return c
	}

	for n, v := range tctx.Header {
		lower := strings.ToLower(n)
		if forwardHeaders.Has(lower) {
			sendingCTX = addHeader(sendingCTX, n, v)
			continue
		}
		for _, prefix := range forwardPrefixes {
			if strings.HasPrefix(lower, prefix) {
				sendingCTX = addHeader(sendingCTX, n, v)
				break
			}
		}
	}
	return sendingCTX
}
