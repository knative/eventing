/*
Copyright 2019 The Knative Authors

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

package broker

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/client"
	cetypes "github.com/cloudevents/sdk-go/v2/types"
	"go.uber.org/zap"
)

const (
	// TTLAttribute is the name of the CloudEvents extension attribute used to store the
	// Broker's TTL (number of times a single events can reply through a Broker continuously). All
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

// TTLDefaulter returns a cloudevents events defaulter that will manage the TTL
// for events with the following rules:
//
//	If TTL is not found, it will set it to the default passed in.
//	If TTL is <= 0, it will remain 0.
//	If TTL is > 1, it will be reduced by one.
func TTLDefaulter(logger *zap.Logger, defaultTTL int32) client.EventDefaulter {
	return func(ctx context.Context, event cloudevents.Event) cloudevents.Event {
		// Get the current or default TTL from the events.
		var ttl int32
		if ttlraw, err := event.Context.GetExtension(TTLAttribute); err != nil {
			logger.Debug("TTL not found in outbound events, defaulting.",
				zap.String("events.id", event.ID()),
				zap.Int32(TTLAttribute, defaultTTL),
				zap.Error(err),
			)
			ttl = defaultTTL
		} else if ttl, err = cetypes.ToInteger(ttlraw); err != nil {
			logger.Warn("Failed to convert existing TTL into integer, defaulting.",
				zap.String("events.id", event.ID()),
				zap.Any(TTLAttribute, ttlraw),
				zap.Error(err),
			)
			ttl = defaultTTL
		} else {
			// Decrement TTL.
			ttl = ttl - 1
			if ttl < 0 {
				ttl = 0
			}
		}
		// Overwrite the TTL into the events.
		if err := SetTTL(event.Context, ttl); err != nil {
			logger.Error("Failed to set TTL on outbound events.",
				zap.String("events.id", event.ID()),
				zap.Int32(TTLAttribute, ttl),
				zap.Error(err),
			)
		}

		return event
	}
}
