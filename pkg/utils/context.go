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

package utils

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go"
	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"k8s.io/apimachinery/pkg/util/sets"
)

// TODO add configurable whitelisting of propagated headers/prefixes (configmap?)

var (
	// These MUST be lowercase strings, as they will be compared against lowercase strings.
	forwardHeaders = sets.NewString(
		// tracing
		"x-request-id",
	)
	// These MUST be lowercase strings, as they will be compared against lowercase strings.
	// Removing CloudEvents ce- prefixes on purpose as they should be set in the CloudEvent itself as extensions.
	// Then the SDK will set them as ce- headers when sending them through HTTP. Otherwise, when using replies we would
	// duplicate ce- headers.
	forwardPrefixes = []string{
		// knative
		"knative-",
	}
)

// ContextFrom creates the context to use when sending a Cloud Event with cloudevents.Client. It
// sets the target if specified, and attaches a filtered set of headers from the initial request.
func ContextFrom(tctx cloudevents.HTTPTransportContext, targetURI *url.URL) context.Context {
	// Get the allowed set of headers.
	h := PassThroughHeaders(tctx.Header)
	// Override the headers.
	tctx.Header = h
	// Create the sending context with the overridden transport context.
	// TODO use the current context here instead of context.Background. Issue: https://github.com/knative/eventing/issues/1953
	//  The reason we are using context.Background is that there is no easy way in the sdk to override
	//  headers, and they will all be passed through. Also note that the sdk does not use the headers from
	//  the transport context to set the request headers.
	//  Further, in the case of replies, the sdk creates the reply context based on the sending context,
	//  thus it ends up adding more headers to the sending context.
	sendingCTX := cehttp.WithTransportContext(context.Background(), tctx)

	for n, v := range h {
		for _, iv := range v {
			sendingCTX = cloudevents.ContextWithHeader(sendingCTX, n, iv)
		}
	}

	if targetURI != nil {
		sendingCTX = cloudevents.ContextWithTarget(sendingCTX, targetURI.String())
	}

	return sendingCTX
}

// PassThroughHeaders extracts the headers from headers that are in the `forwardHeaders` set
// or has any of the prefixes in `forwardPrefixes`.
func PassThroughHeaders(headers http.Header) http.Header {
	h := http.Header{}

	for n, v := range headers {
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
