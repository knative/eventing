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
	"github.com/cloudevents/sdk-go/pkg/cloudevents"
)

const (
	// V02TTLAttribute is the name of the CloudEvents 0.2 extension attribute used to store the
	// Broker's TTL (number of times a single event can reply through a Broker continuously).
	V02TTLAttribute = "knativebrokerttl"
)

// SetTTL sets the TTL into the EventContext. ttl should be a positive integer.
func SetTTL(ctx cloudevents.EventContext, ttl interface{}) cloudevents.EventContext {
	v2 := ctx.AsV02()
	if v2.Extensions == nil {
		v2.Extensions = make(map[string]interface{})
	}
	v2.Extensions[V02TTLAttribute] = ttl
	return v2
}
