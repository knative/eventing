/*
Copyright 2020 The Knative Authors

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

// Package observer is intended to be a utility that can observe CloudEvents as
// a Sink (addressable + CloudEvents) and fan out the observed events to
// several EventLog implementations if required.
//
// The intention is something like:
//
//                                 +--> [Kubernetes Events via recorder]
//                                 |
// [Event Producer] --> [Observer] +--> [pod logs via writer]
//                                 |
//                                 +--> [http forward via http]
//
// Then we can collect all events observed with something like:
//
// [Kubernetes Events] <-- [collector]
//
package observer
