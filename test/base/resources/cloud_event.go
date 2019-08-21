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

package resources

// CloudEvent specifies the arguments for a CloudEvent used by the sendevents or transformevents image.
type CloudEvent struct {
	ID         string
	Type       string
	Source     string
	Extensions map[string]interface{}
	Data       string // must be in json format
	Encoding   string // binary or structured
}

// CloudEventBaseData defines a simple struct that can be used as data of a CloudEvent.
type CloudEventBaseData struct {
	Sequence int    `json:"id"`
	Message  string `json:"message"`
}

// CloudEvent related constants.
const (
	CloudEventEncodingBinary     = "binary"
	CloudEventEncodingStructured = "structured"
	CloudEventDefaultEncoding    = CloudEventEncodingBinary
	CloudEventDefaultType        = "dev.knative.test.event"
)
