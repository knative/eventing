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
	// v02TTLAttribute is the name of the CloudEvents 0.2 extension attribute used to store the
	// Broker's TTL (number of times a single event can reply through a Broker continuously). All
	// interactions with the attribute should be done through the GetTTL and SetTTL functions.
	v02TTLAttribute = "knativebrokerttl"

	v02UniqueTriggerAttribute = "knativeuniquetrigger"
)

// GetTTL finds the TTL in the EventContext using a case insensitive comparison
// for the key. The second return param, is the case preserved key that matched.
// Depending on the encoding/transport, the extension case could be changed.
func GetTTL(ctx cloudevents.EventContext) (interface{}, string) {
	return getAttribute(ctx, v02TTLAttribute)
}

// SetTTL sets the TTL into the EventContext. ttl should be a positive integer.
func SetTTL(ctx cloudevents.EventContext, ttl interface{}) (cloudevents.EventContext, error) {
	return setAttribute(ctx, v02TTLAttribute, ttl)
}

func getAttribute(ctx cloudevents.EventContext, attribute string) (interface{}, string) {
	attribute = strings.ToLower(attribute)
	for k, v := range ctx.AsV02().Extensions {
		if lower := strings.ToLower(k); lower == attribute {
			return v, k
		}
	}
	return nil, attribute
}

func setAttribute(ctx cloudevents.EventContext, attribute string, value interface{}) (cloudevents.EventContext, error) {
	err := ctx.SetExtension(attribute, value)
	return ctx, err
}

func GetUniqueTrigger(ctx cloudevents.EventContext) string {
	v, _ := getAttribute(ctx, v02UniqueTriggerAttribute)
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func SetUniqueTrigger(ctx cloudevents.EventContext, trigger string) (cloudevents.EventContext, error) {
	return setAttribute(ctx, v02UniqueTriggerAttribute, trigger)
}
