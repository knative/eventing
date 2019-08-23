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
	"net/http"
	"net/url"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go"
	"k8s.io/apimachinery/pkg/util/sets"
)

var (
	// These MUST be lowercase strings, as they will be compared against lowercase strings.
	forwardHeaders = sets.NewString(
		// tracing
		"x-request-id",
		// Single header for b3 tracing. See
		// https://github.com/openzipkin/b3-propagation#single-header.
		"b3",
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
func SendingContext(ctx context.Context, tctx cloudevents.HTTPTransportContext, targetURI *url.URL) context.Context {
	sendingCTX := cloudevents.ContextWithTarget(ctx, targetURI.String())

	h := ExtractPassThroughHeaders(tctx)
	for n, v := range h {
		for _, iv := range v {
			sendingCTX = cloudevents.ContextWithHeader(sendingCTX, n, iv)
		}
	}

	return sendingCTX
}

// ExtractPassThroughHeaders extracts the headers that are in the `forwardHeaders` set
// or has any of the prefixes in `forwardPrefixes`.
func ExtractPassThroughHeaders(tctx cloudevents.HTTPTransportContext) http.Header {
	h := http.Header{}

	for n, v := range tctx.Header {
		lower := strings.ToLower(n)
		if forwardHeaders.Has(lower) {
			h[n] = v
			continue
		}
		for _, prefix := range forwardPrefixes {
			if strings.HasPrefix(lower, prefix) {
				h[n] = v
				break
			}
		}
	}
	return h
}
