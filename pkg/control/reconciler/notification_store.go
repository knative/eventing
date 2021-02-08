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

package reconciler

import (
	"context"
	"reflect"
	"sync"

	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/control"
)

type NotificationStore struct {
	enqueueKey func(name types.NamespacedName)

	payloadParser PayloadParser

	// Map indexed with types.NamespacedName and another sync map as values
	notificationStoreMutex sync.RWMutex
	notificationStore      map[types.NamespacedName]map[string]interface{}
}

type PayloadParser func([]byte) (interface{}, error)

type ValueMerger func(old interface{}, new interface{}) interface{}

func PassNewValue(old interface{}, new interface{}) interface{} {
	return new
}

func NewNotificationStore(enqueueKey func(name types.NamespacedName), parser PayloadParser) *NotificationStore {
	return &NotificationStore{
		enqueueKey:        enqueueKey,
		payloadParser:     parser,
		notificationStore: make(map[types.NamespacedName]map[string]interface{}),
	}
}

func (n *NotificationStore) ControlMessageHandler(srcName types.NamespacedName, pod string, valueMerger ValueMerger) control.MessageHandler {
	return control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
		// Parse the payload
		parsedPayload, err := n.payloadParser(message.Payload())
		if err != nil {
			logging.FromContext(ctx).Errorf(
				"Cannot parse the payload of the received message with opcode %d (sounds like a programming error of the adapter): %w",
				message.Headers().OpCode(),
				err,
			)
			return
		}

		// Store new value in the notification store
		shouldRunReconcile := n.storeNewValue(srcName, pod, valueMerger, parsedPayload)

		// Ack the message
		message.Ack()

		if shouldRunReconcile {
			// Trigger reconciler
			n.enqueueKey(srcName)
		}
	})
}

func (n *NotificationStore) GetPodsNotifications(srcName types.NamespacedName) (map[string]interface{}, bool) {
	n.notificationStoreMutex.RLock()
	defer n.notificationStoreMutex.RUnlock()
	pods, ok := n.notificationStore[srcName]
	if !ok {
		return nil, false
	}
	return copyMap(pods), true
}

func (n *NotificationStore) GetPodNotification(srcName types.NamespacedName, pod string) (interface{}, bool) {
	n.notificationStoreMutex.RLock()
	defer n.notificationStoreMutex.RUnlock()
	podsNotifications, ok := n.notificationStore[srcName]
	if !ok {
		return nil, false
	}
	val, ok := podsNotifications[pod]
	return val, ok
}

func (n *NotificationStore) CleanPodsNotifications(srcName types.NamespacedName) {
	n.notificationStoreMutex.Lock()
	defer n.notificationStoreMutex.Unlock()
	delete(n.notificationStore, srcName)
}

func (n *NotificationStore) CleanPodNotification(srcName types.NamespacedName, pod string) {
	n.notificationStoreMutex.Lock()
	defer n.notificationStoreMutex.Unlock()
	podsNotifications, ok := n.notificationStore[srcName]
	if !ok {
		return
	}

	delete(podsNotifications, pod)
	if len(podsNotifications) == 0 {
		delete(n.notificationStore, srcName)
	}
}

func (n *NotificationStore) storeNewValue(srcName types.NamespacedName, pod string, valueMerger ValueMerger, newValue interface{}) bool {
	n.notificationStoreMutex.Lock()
	defer n.notificationStoreMutex.Unlock()

	podsNotifications, ok := n.notificationStore[srcName]
	if !ok {
		podsNotifications = make(map[string]interface{})
		n.notificationStore[srcName] = podsNotifications
	}

	oldValue, ok := podsNotifications[pod]
	valueToStore := oldValue
	if ok {
		valueToStore = valueMerger(valueToStore, newValue)
	} else {
		valueToStore = newValue
	}

	if valueToStore != nil {
		podsNotifications[pod] = valueToStore
		return !reflect.DeepEqual(oldValue, valueToStore)
	} else {
		delete(podsNotifications, pod)

		if len(podsNotifications) == 0 {
			// We can remove the podsNotifications map now
			delete(n.notificationStore, srcName)
		}
		return true
	}
}

func copyMap(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
