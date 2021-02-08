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

package service

import (
	"context"

	"knative.dev/pkg/logging"

	ctrl "knative.dev/eventing/pkg/control"
)

// NoopMessageHandler is a default noop message handler that logs the message and acks it.
var NoopMessageHandler ctrl.MessageHandlerFunc = func(ctx context.Context, message ctrl.ServiceMessage) {
	logging.FromContext(ctx).Warnf("Discarding control message '%s'", message.Headers().UUID())
	message.Ack()
}

// LoggerErrorHandler is a default noop error handler that logs the error.
var LoggerErrorHandler ctrl.ErrorHandlerFunc = func(ctx context.Context, err error) {
	logging.FromContext(ctx).Debugf("Error from the connection: %s", err)
}
