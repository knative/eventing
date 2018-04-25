/*
Copyright 2018 Google, Inc. All rights reserved.

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

package action

import (
	"encoding/json"

	"github.com/elafros/eventing/pkg/event"
	"github.com/golang/glog"
)

const (
	// LoggingActionType is the expected Bind Processor type
	// that will cause events to be logged and dropped.
	LoggingActionType = "eventing.elafros.dev/EventLogger"
)

// A LoggingAction will log and drop Events.
type LoggingAction struct{}

// SendEvent implements Action.SendEvent
func (a LoggingAction) SendEvent(name string, data interface{}, context *event.Context) (interface{}, error) {
	event := map[string]interface{}{
		"data":    data,
		"context": context,
	}
	b, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	glog.Infof("[%s] sendEvent %s", name, string(b))
	return nil, nil
}

// NewLoggingAction creates an Action that will log and drop Events.
func NewLoggingAction() LoggingAction {
	return LoggingAction{}
}
