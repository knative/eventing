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
	"strings"

	cloudevents "github.com/cloudevents/sdk-go"
)

const (
	// V03TTLAttribute is the name of the CloudEvents 0.3 extension attribute used to store the
	// Broker's TTL (number of times a single event can reply through a Broker continuously). All
	// interactions with the attribute should be done through the GetTTL and SetTTL functions.
	V03TTLAttribute = "knativebrokerttl"
)

// GetTTL finds the TTL in the EventContext using a case insensitive comparison
// for the key. The second return param, is the case preserved key that matched.
// Depending on the encoding/transport, the extension case could be changed.
func GetTTL(ctx cloudevents.EventContext) (interface{}, string) {
	for k, v := range ctx.AsV03().Extensions {
		if lower := strings.ToLower(k); lower == V03TTLAttribute {
			return v, k
		}
	}
	return nil, V03TTLAttribute
}

// SetTTL sets the TTL into the EventContext. ttl should be a positive integer.
func SetTTL(ctx cloudevents.EventContext, ttl interface{}) (cloudevents.EventContext, error) {
	err := ctx.SetExtension(V03TTLAttribute, ttl)
	return ctx, err
}
