/*
Copyright 2020 The Knative Authors

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
	"strings"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/cloudevents/sdk-go/v2/types"
)

const (
	// AcceptReplyHeaderKey is the key for the event reply header.
	// https://github.com/knative/eventing/blob/master/docs/spec/data-plane.md#event-reply-contract
	AcceptReplyHeaderKey = "Prefer"
	// AcceptReply is the value to accept event reply.
	AcceptReply = "reply"
)

var acceptReplyHeaders = []string{AcceptReply}

// hasToken reports whether token appears with v, ASCII
// case-insensitive, with user defined boundaries.
// token must be all lowercase.
// v may contain mixed cased.
// The function is modified from the standard http header parsing: https://golang.org/src/net/http/header.go
func hasToken(v, token string, isTokenBoundary func(byte) bool) bool {
	if len(token) > len(v) || token == "" {
		return false
	}
	if v == token {
		return true
	}
	for sp := 0; sp <= len(v)-len(token); sp++ {
		// Check that first character is good.
		// The token is ASCII, so checking only a single byte
		// is sufficient. We skip this potential starting
		// position if both the first byte and its potential
		// ASCII uppercase equivalent (b|0x20) don't match.
		// False positives ('^' => '~') are caught by EqualFold.
		if b := v[sp]; b != token[0] && b|0x20 != token[0] {
			continue
		}
		// Check that start pos is on a valid token boundary.
		if sp > 0 && !isTokenBoundary(v[sp-1]) {
			continue
		}
		// Check that end pos is on a valid token boundary.
		if endPos := sp + len(token); endPos != len(v) && !isTokenBoundary(v[endPos]) {
			continue
		}
		if strings.EqualFold(v[sp:sp+len(token)], token) {
			return true
		}
	}
	return false
}

// SetAcceptReplyHeader sets Prefer: reply.
func SetAcceptReplyHeader(headers nethttp.Header) {
	if _, ok := headers[AcceptReplyHeaderKey]; !ok {
		headers[AcceptReplyHeaderKey] = acceptReplyHeaders
		return
	}
	for _, f := range headers[AcceptReplyHeaderKey] {
		existed := hasToken(f, AcceptReply, func(b byte) bool {
			return b == ' ' || b == ',' || b == '\t' || b == ';' || b == '='
		})
		// If token already existed, simply return
		if existed {
			return
		}
	}
	headers[AcceptReplyHeaderKey] = append(headers[AcceptReplyHeaderKey], AcceptReply)
}

func WriteHTTPRequestWithAdditionalHeaders(ctx context.Context, message binding.Message, req *nethttp.Request,
	additionalHeaders nethttp.Header, transformers ...binding.Transformer) error {
	err := http.WriteRequest(ctx, message, req, transformers...)
	if err != nil {
		return err
	}

	for k, v := range additionalHeaders {
		req.Header[k] = v
	}

	return nil
}

type TypeExtractorTransformer string

func (a *TypeExtractorTransformer) Transform(reader binding.MessageMetadataReader, _ binding.MessageMetadataWriter) error {
	_, ty := reader.GetAttribute(spec.Type)
	if ty != nil {
		tyParsed, err := types.ToString(ty)
		if err != nil {
			return err
		}
		*a = TypeExtractorTransformer(tyParsed)
	}
	return nil
}
