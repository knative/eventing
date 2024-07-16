/*
Copyright 2021 The Knative Authors

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

package sender

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// Sender will send messages continuously until process receives a SIGINT
type Sender interface {
	SendContinually()
}

type EndpointSupporter interface {
	// Supports will check given endpoint definition and decide if it's valid for
	// this sender.
	Supports(endpoint interface{}) bool
}

// EventSenderWithContext will be used to send events to configured endpoint, passing a context.
type EventSenderWithContext interface {
	EndpointSupporter
	// SendEventWithContext will send event to the given endpoint and pass context.
	SendEventWithContext(ctx context.Context, ce cloudevents.Event, endpoint interface{}) error
}
