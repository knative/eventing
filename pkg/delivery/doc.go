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

// Package delivery implements an event delivery service. The Receiver
// interface exposes a WebHook where compatible Events can be sent.
// The Receiver enqueues the Event and then acknowledges the webhook.
// A Sender polls the events Queue and delivers the event to the
// appropriate Action according to the Bind that matched the Event.
package delivery
