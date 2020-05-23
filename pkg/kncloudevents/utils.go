/*
 * Copyright 2020 The Knative Authors
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

package kncloudevents

import (
	"context"
	nethttp "net/http"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
)

func WriteHttpRequestWithAdditionalHeaders(ctx context.Context, message binding.Message, req *nethttp.Request, additionalHeaders nethttp.Header, transformers ...binding.Transformer) error {
	err := http.WriteRequest(ctx, message, req, transformers...)
	if err != nil {
		return err
	}

	for k, v := range additionalHeaders {
		req.Header[k] = v
	}

	return nil
}
