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
	cloudevents "github.com/cloudevents/sdk-go"
	cetypes "github.com/cloudevents/sdk-go/pkg/cloudevents/types"
)

const (
	// TTLAttribute is the name of the CloudEvents extension attribute used to store the
	// Broker's TTL (number of times a single event can reply through a Broker continuously). All
	// interactions with the attribute should be done through the GetTTL and SetTTL functions.
	TTLAttribute = "knativebrokerttl"
)

// GetTTL finds the TTL in the EventContext using a case insensitive comparison
// for the key. The second return param, is the case preserved key that matched.
// Depending on the encoding/transport, the extension case could be changed.
func GetTTL(ctx cloudevents.EventContext) (int32, error) {
	ttl, err := ctx.GetExtension(TTLAttribute)
	if err != nil {
		return 0, err
	}
	return cetypes.ToInteger(ttl)
}

// SetTTL sets the TTL into the EventContext. ttl should be a positive integer.
func SetTTL(ctx cloudevents.EventContext, ttl int32) error {
	return ctx.SetExtension(TTLAttribute, ttl)
}

// DeleteTTL removes the TTL CE extension attribute
func DeleteTTL(ctx cloudevents.EventContext) error {
	return ctx.SetExtension(TTLAttribute, nil)
}
