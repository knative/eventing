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

package test

// Helper struct to easily create subscriber pods, services and triggers.
// We also use this to verify the expected events that should be received
// by the particular subscriber pods.
type DumperInfo struct {
	Namespace   string
	Broker      string
	EventType   string
	EventSource string
	Selector    map[string]string
	Expect      []string
}

// Helper struct to easily create event sender pods.
type SenderInfo struct {
	Namespace   string
	Url         string
	EventType   string
	EventSource string
}
